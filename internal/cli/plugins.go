package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"gust/internal/adapters/wire"
	"gust/internal/registry"
)

// loadJudgePlugin starts a Tier-2 wire plugin intended to provide llm_judge
// (typically a Python script wrapping an official provider SDK). The process
// stays alive for the CLI invocation; registry override wins over the Go builtin.
func loadJudgePlugin(pluginSpec string) (func(), error) {
	if strings.TrimSpace(pluginSpec) == "" {
		return func() {}, nil
	}
	parts := strings.Fields(pluginSpec)
	if len(parts) == 0 {
		return func() {}, nil
	}
	cmd := parts[0]
	args := parts[1:]
	// Convenience: bare .py path → python interpreter
	if len(parts) == 1 && strings.HasSuffix(strings.ToLower(parts[0]), ".py") {
		cmd = "python"
		if _, err := os.Stat(parts[0]); err != nil {
			cmd = "python3"
		}
		args = []string{parts[0]}
	}

	proc, err := wire.StartJudgePlugin(context.Background(), cmd, args...)
	if err != nil {
		return nil, fmt.Errorf("start judge plugin: %w", err)
	}
	ev := wire.NewWireEvaluator(proc)
	registry.DefaultRegistry.ReplaceEvaluator(ev)
	return func() { _ = proc.Close() }, nil
}
