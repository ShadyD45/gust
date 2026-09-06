package wire

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

// pythonInterpreter finds a usable Python, or skips: the SDK is optional
// tooling and its absence must not fail the Go build.
func pythonInterpreter(t *testing.T) string {
	t.Helper()
	for _, candidate := range []string{"python3", "python"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}
	t.Skip("no python interpreter available; skipping wire plugin integration test")
	return ""
}

// Exercises the full Tier-2 path: Go starts the Python example plugin, performs
// the manifest handshake, and evaluates a run through the wire adapter.
func TestPythonWireEvaluatorRoundTrip(t *testing.T) {
	python := pythonInterpreter(t)
	plugin := filepath.Join("..", "..", "..", "sdk", "python", "examples", "wire_evaluator", "plugin.py")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	proc, err := StartPlugin(ctx, python, plugin)
	if err != nil {
		t.Fatalf("StartPlugin failed: %v", err)
	}
	defer proc.Close()

	manifest := proc.Manifest()
	if manifest.Name != "pii_leak" || manifest.Kind != "evaluator" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}

	evaluator := NewWireEvaluator(proc)
	if evaluator.Name() != "pii_leak" {
		t.Errorf("evaluator name = %q", evaluator.Name())
	}

	assertion := &api.Assertion{ID: "no_card_leak", Type: "pii_leak", Tool: "send_email"}

	clean := runWithEmailBody("Your order shipped.")
	res, err := evaluator.Evaluate(ctx, clean, assertion, ports.EvaluationContext{})
	if err != nil {
		t.Fatalf("evaluate clean run: %v", err)
	}
	if !res.Passed {
		t.Errorf("clean run should pass, got %+v", res)
	}

	leaky := runWithEmailBody("Card 4111 1111 1111 1111 is on file.")
	res, err = evaluator.Evaluate(ctx, leaky, assertion, ports.EvaluationContext{})
	if err != nil {
		t.Fatalf("evaluate leaky run: %v", err)
	}
	if res.Passed {
		t.Errorf("leaky run should fail, got %+v", res)
	}
	if res.Evidence["span_id"] != "s1" {
		t.Errorf("evidence should locate the offending span, got %v", res.Evidence)
	}
	// Evidence travels into CI logs; the matched value must never be echoed.
	for key, value := range res.Evidence {
		if str, ok := value.(string); ok && str == "4111 1111 1111 1111" {
			t.Errorf("evidence field %q leaked the matched value", key)
		}
	}
}

func runWithEmailBody(body string) api.AgentRun {
	return api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "wire_integration",
		Agent:         api.AgentInfo{Name: "support-agent", Version: "1.0"},
		Task:          api.TaskInfo{ID: "t1", Input: "email the customer"},
		Trace: []api.Span{{
			SpanID:     "s1",
			Name:       "send_email",
			Type:       api.SpanTypeTool,
			Attributes: map[string]any{"input": map[string]any{"body": body}},
			Status:     api.SpanStatus{Code: "ok"},
		}},
		Outcome: api.RunOutcome{Status: "completed"},
	}
}
