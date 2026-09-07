package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"gust/internal/ports"
	"gust/internal/registry"
	"gust/pkg/api"
)

func TestParsePluginSpec(t *testing.T) {
	got, err := parsePluginSpec("python plugin.py")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Command) != 2 || got.Command[1] != "plugin.py" || got.Name != "" {
		t.Fatalf("got %+v", got)
	}

	got, err = parsePluginSpec("pii_leak=python plugin.py")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "pii_leak" || got.Command[0] != "python" {
		t.Fatalf("alias: %+v", got)
	}

	got, err = parsePluginSpec("sdk/python/examples/wire_evaluator/plugin.py")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Command) != 2 || got.Command[1] != "sdk/python/examples/wire_evaluator/plugin.py" {
		t.Fatalf("bare py: %+v", got)
	}
}

func TestPluginsFromConfigResolvesRelative(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "plugin.py")
	if err := os.WriteFile(script, []byte("# plugin\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loads := pluginsFromConfig([]ProjectPlugin{{
		Command: []string{"python", "plugin.py"},
		Name:    "custom",
		Role:    "evaluator",
	}}, dir)
	if len(loads) != 1 {
		t.Fatalf("len=%d", len(loads))
	}
	if loads[0].Command[1] != script {
		t.Fatalf("resolved %q want %q", loads[0].Command[1], script)
	}
	if loads[0].Name != "custom" {
		t.Fatalf("name=%s", loads[0].Name)
	}
}

type stubEval struct{ n string }

func (s stubEval) Name() string    { return s.n }
func (s stubEval) Version() string { return "1" }
func (s stubEval) Evaluate(context.Context, api.AgentRun, *api.Assertion, ports.EvaluationContext) (ports.EvaluationResult, error) {
	return ports.EvaluationResult{Passed: true}, nil
}

func TestNamedEvaluatorAlias(t *testing.T) {
	ev := namedEvaluator{name: "openai_judge", inner: stubEval{n: "llm_judge"}}
	if ev.Name() != "openai_judge" {
		t.Fatalf("name=%s", ev.Name())
	}
	res, err := ev.Evaluate(context.Background(), api.AgentRun{}, nil, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("evaluate: %+v %v", res, err)
	}
}

func TestPythonPluginLoadAndAlias(t *testing.T) {
	if _, err := exec.LookPath("python"); err != nil {
		if _, err := exec.LookPath("python3"); err != nil {
			t.Skip("no python")
		}
	}
	root := repoRoot(t)
	plugin := filepath.Join(root, "sdk", "python", "examples", "wire_evaluator", "plugin.py")
	cleanup, err := loadEvalPlugins(nil, "", []string{"pii_leak=" + plugin}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	ev, ok := registry.DefaultRegistry.GetEvaluator("pii_leak")
	if !ok {
		t.Fatal("pii_leak not registered")
	}
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "p",
		Agent:         api.AgentInfo{Name: "a", Version: "1"},
		Task:          api.TaskInfo{ID: "t", Input: "x"},
		Trace: []api.Span{{
			SpanID:     "s1",
			Name:       "send_email",
			Type:       api.SpanTypeTool,
			Attributes: map[string]any{"input": map[string]any{"body": "Card 4111 1111 1111 1111"}},
			Status:     api.SpanStatus{Code: "ok"},
		}},
		Outcome: api.RunOutcome{Status: "completed"},
	}
	res, err := ev.Evaluate(context.Background(), run, &api.Assertion{ID: "a", Type: "pii_leak", Tool: "send_email"}, ports.EvaluationContext{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Passed {
		t.Fatal("expected pii leak to fail")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
