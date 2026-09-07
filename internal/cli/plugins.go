package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gust/internal/adapters/wire"
	"gust/internal/ports"
	"gust/internal/registry"
	"gust/pkg/api"
)

// ProjectPlugin is a gust.yaml plugin entry.
type ProjectPlugin struct {
	Command []string `yaml:"command"`
	Name    string   `yaml:"name"`
	Role    string   `yaml:"role"` // evaluator | judge
}

type pluginLoad struct {
	Command []string
	Name    string
	Role    string
	Replace bool
}

type namedEvaluator struct {
	name  string
	inner ports.Evaluator
}

func (e namedEvaluator) Name() string { return e.name }
func (e namedEvaluator) Version() string {
	return e.inner.Version()
}
func (e namedEvaluator) Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx ports.EvaluationContext) (ports.EvaluationResult, error) {
	return e.inner.Evaluate(ctx, run, expected, evalCtx)
}

// parsePluginSpec parses `--plugin` values:
//
//	python plugin.py
//	alias=python plugin.py
//	plugin.py                 (python convenience)
func parsePluginSpec(raw string) (pluginLoad, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return pluginLoad{}, fmt.Errorf("empty plugin spec")
	}
	name := ""
	if i := strings.IndexByte(raw, '='); i > 0 {
		head := raw[:i]
		if !strings.ContainsAny(head, `/\:`) && !strings.Contains(head, " ") && !strings.HasSuffix(strings.ToLower(head), ".py") {
			name = strings.TrimSpace(head)
			raw = strings.TrimSpace(raw[i+1:])
		}
	}
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return pluginLoad{}, fmt.Errorf("empty plugin command")
	}
	if len(parts) == 1 && strings.HasSuffix(strings.ToLower(parts[0]), ".py") {
		parts = []string{pythonBin(parts[0]), parts[0]}
	}
	return pluginLoad{Command: parts, Name: name, Role: "evaluator"}, nil
}

func pythonBin(script string) string {
	if _, err := os.Stat(script); err != nil {
		if _, err := exec.LookPath("python3"); err == nil {
			return "python3"
		}
	}
	if _, err := exec.LookPath("python"); err == nil {
		return "python"
	}
	return "python3"
}

func resolvePluginCommand(cmd []string, baseDir string) []string {
	out := append([]string(nil), cmd...)
	if len(out) == 0 {
		return out
	}
	idx := 0
	if len(out) > 1 && (strings.EqualFold(filepath.Base(out[0]), "python") || strings.EqualFold(filepath.Base(out[0]), "python3") || strings.EqualFold(filepath.Base(out[0]), "python.exe")) {
		idx = 1
	}
	if idx < len(out) && !filepath.IsAbs(out[idx]) && baseDir != "" {
		candidate := filepath.Join(baseDir, out[idx])
		if _, err := os.Stat(candidate); err == nil {
			out[idx] = candidate
		}
	}
	if len(out) == 1 && strings.HasSuffix(strings.ToLower(out[0]), ".py") {
		out = []string{pythonBin(out[0]), out[0]}
	}
	return out
}

func pluginsFromConfig(cfgPlugins []ProjectPlugin, cfgDir string) []pluginLoad {
	out := make([]pluginLoad, 0, len(cfgPlugins))
	for _, p := range cfgPlugins {
		if len(p.Command) == 0 {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(p.Role))
		if role == "" {
			role = "evaluator"
		}
		out = append(out, pluginLoad{
			Command: resolvePluginCommand(p.Command, cfgDir),
			Name:    p.Name,
			Role:    role,
			Replace: false,
		})
	}
	return out
}

func pluginsFromFlags(specs []string) ([]pluginLoad, error) {
	out := make([]pluginLoad, 0, len(specs))
	for _, spec := range specs {
		if strings.TrimSpace(spec) == "" {
			continue
		}
		pl, err := parsePluginSpec(spec)
		if err != nil {
			return nil, err
		}
		pl.Replace = true
		out = append(out, pl)
	}
	return out, nil
}

func loadEvalPlugins(cfgPlugins []ProjectPlugin, cfgDir string, flagSpecs []string, judgePlugin string) (func(), error) {
	loads := pluginsFromConfig(cfgPlugins, cfgDir)
	fromFlags, err := pluginsFromFlags(flagSpecs)
	if err != nil {
		return func() {}, err
	}
	loads = append(loads, fromFlags...)
	if strings.TrimSpace(judgePlugin) != "" {
		pl, err := parsePluginSpec(judgePlugin)
		if err != nil {
			return func() {}, err
		}
		pl.Role = "judge"
		pl.Replace = true
		if pl.Name == "" {
			pl.Name = "llm_judge"
		}
		loads = append(loads, pl)
	}
	if len(loads) == 0 {
		return func() {}, nil
	}

	var closers []func()
	cleanup := func() {
		for i := len(closers) - 1; i >= 0; i-- {
			closers[i]()
		}
	}
	seen := map[string]struct{}{}
	flagSeen := map[string]struct{}{}

	for _, pl := range loads {
		cmd := pl.Command
		if len(cmd) == 0 {
			cleanup()
			return func() {}, fmt.Errorf("plugin command is required")
		}
		role := strings.ToLower(pl.Role)
		if role == "" {
			role = "evaluator"
		}
		start := wire.StartPlugin
		if role == "judge" {
			start = wire.StartJudgePlugin
		}
		proc, err := start(context.Background(), cmd[0], cmd[1:]...)
		if err != nil {
			cleanup()
			return func() {}, fmt.Errorf("start plugin %v: %w", cmd, err)
		}
		closers = append(closers, func() { _ = proc.Close() })

		manifest := proc.Manifest()
		kind := strings.ToLower(manifest.Kind)
		if kind != "" && kind != "evaluator" {
			cleanup()
			return func() {}, fmt.Errorf("plugin %q has kind %q; only evaluator plugins are supported", manifest.Name, manifest.Kind)
		}
		ev := ports.Evaluator(wire.NewWireEvaluator(proc))
		name := pl.Name
		if name == "" {
			name = ev.Name()
		}
		if name != ev.Name() {
			ev = namedEvaluator{name: name, inner: ev}
		}
		if _, dup := seen[name]; dup && !pl.Replace {
			cleanup()
			return func() {}, fmt.Errorf("duplicate plugin name %q", name)
		}
		if pl.Replace {
			if _, dup := flagSeen[name]; dup {
				cleanup()
				return func() {}, fmt.Errorf("duplicate --plugin name %q", name)
			}
			flagSeen[name] = struct{}{}
		}
		seen[name] = struct{}{}
		registry.DefaultRegistry.ReplaceEvaluator(ev)
	}
	return cleanup, nil
}

func loadPluginsForCommand(flagSpecs []string, judgePlugin string) (func(), error) {
	cfg, cfgPath, err := loadProjectConfig("")
	if err != nil {
		return func() {}, err
	}
	cfgDir := ""
	if cfgPath != "" {
		cfgDir = filepath.Dir(cfgPath)
	}
	return loadEvalPlugins(cfg.Plugins, cfgDir, flagSpecs, judgePlugin)
}

// loadJudgePlugin is the backward-compatible alias used before --plugin existed.
func loadJudgePlugin(pluginSpec string) (func(), error) {
	return loadPluginsForCommand(nil, pluginSpec)
}
