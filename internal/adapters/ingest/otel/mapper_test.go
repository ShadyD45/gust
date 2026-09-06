package otel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gust/internal/adapters/evaluators"
	"gust/internal/core/analyze"
	"gust/internal/ports"
	"gust/pkg/api"
	"gust/pkg/jcs"
)

func goldenExport(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", "testdata", "otel", "openinference_cancel.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden export: %v", err)
	}
	return data
}

func TestMapGoldenExport(t *testing.T) {
	run, err := NewMapper().MapBytes(goldenExport(t), Options{})
	if err != nil {
		t.Fatalf("MapBytes failed: %v", err)
	}

	if run.RunID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("run id = %q", run.RunID)
	}
	if run.Agent.Name != "support-agent" || run.Agent.Version != "1.4" {
		t.Errorf("agent = %+v", run.Agent)
	}
	if run.Task.ID != "refund-001" || run.Task.Input != "Cancel my latest order" {
		t.Errorf("task = %+v", run.Task)
	}
	if run.Outcome.Status != "completed" {
		t.Errorf("outcome status = %q", run.Outcome.Status)
	}
	if run.Outcome.Output != "Order 123 cancelled successfully." {
		t.Errorf("outcome output = %q", run.Outcome.Output)
	}
	if got, want := run.Outcome.DurationNs.Milliseconds(), int64(30); got != want {
		t.Errorf("duration = %d ms, want %d ms", got, want)
	}
	if len(run.Trace) != 3 {
		t.Fatalf("trace length = %d, want 3", len(run.Trace))
	}

	// Tool spans must carry the logical tool name, not the instrumentation label.
	tools := map[string]api.Span{}
	for _, sp := range run.Trace {
		if sp.Type == api.SpanTypeTool {
			tools[sp.Name] = sp
		}
	}
	if len(tools) != 2 {
		t.Fatalf("tool spans = %d, want 2 (%v)", len(tools), tools)
	}

	// JSON-encoded OpenInference parameters must decode into comparable structures.
	args, ok := tools["cancel_order"].Attributes["input"].(map[string]any)
	if !ok {
		t.Fatalf("cancel_order input is %T, want map", tools["cancel_order"].Attributes["input"])
	}
	if args["order_id"] != float64(123) {
		t.Errorf("order_id = %v (%T), want 123", args["order_id"], args["order_id"])
	}
}

func TestMapIsDeterministic(t *testing.T) {
	data := goldenExport(t)
	m := NewMapper()

	first, err := m.MapBytes(data, Options{})
	if err != nil {
		t.Fatalf("first map failed: %v", err)
	}
	firstHash, err := jcs.ContentHash(first)
	if err != nil {
		t.Fatalf("hash first run: %v", err)
	}

	for i := 0; i < 25; i++ {
		next, err := m.MapBytes(data, Options{})
		if err != nil {
			t.Fatalf("map %d failed: %v", i, err)
		}
		hash, err := jcs.ContentHash(next)
		if err != nil {
			t.Fatalf("hash run %d: %v", i, err)
		}
		if hash != firstHash {
			t.Fatalf("content hash drifted on iteration %d: %s != %s", i, hash, firstHash)
		}
	}
}

// An ingested run must satisfy the same assertions as a hand-authored run for
// the same trajectory — otherwise ingestion is not a drop-in capture path.
func TestIngestedRunAnalyzesLikeHandAuthored(t *testing.T) {
	run, err := NewMapper().MapBytes(goldenExport(t), Options{})
	if err != nil {
		t.Fatalf("MapBytes failed: %v", err)
	}

	assertions := []api.Assertion{
		{ID: "a1", Type: api.AssertTaskSuccess, Parameters: map[string]any{"expected_output": "cancelled"}},
		{ID: "a2", Type: api.AssertToolCall, Tool: "get_orders", Arguments: map[string]any{"customer_id": float64(42)}},
		{ID: "a3", Type: api.AssertToolCall, Tool: "cancel_order", Arguments: map[string]any{"order_id": float64(123)}},
		{ID: "a4", Type: api.AssertForbiddenToolCall, Tool: "forbidden_admin_access"},
		{ID: "a5", Type: api.AssertToolSequence, Parameters: map[string]any{"sequence": []any{"get_orders", "cancel_order"}}},
	}

	engine := analyze.NewEngine(evaluators.AllBuiltinEvaluators())
	report, err := engine.AnalyzeRun(context.Background(), run, assertions, ports.EvaluationContext{})
	if err != nil {
		t.Fatalf("AnalyzeRun failed: %v", err)
	}
	if !report.Passed {
		for _, r := range report.Results {
			if !r.Passed {
				t.Errorf("assertion failed: %s: %s (evidence: %v)", r.EvaluatorName, r.Message, r.Evidence)
			}
		}
	}
}

func TestMapErrors(t *testing.T) {
	m := NewMapper()

	if _, err := m.MapBytes(nil, Options{}); !errors.Is(err, ErrEmptyPayload) {
		t.Errorf("empty input error = %v, want ErrEmptyPayload", err)
	}
	if _, err := m.MapBytes([]byte("not json"), Options{}); !errors.Is(err, ErrInvalidPayload) {
		t.Errorf("malformed input error = %v, want ErrInvalidPayload", err)
	}
	if _, err := m.MapBytes([]byte(`{"resourceSpans":[]}`), Options{}); !errors.Is(err, ErrEmptyPayload) {
		t.Errorf("no spans error = %v, want ErrEmptyPayload", err)
	}
	if _, err := m.MapBytes(goldenExport(t), Options{TraceID: "missing"}); !errors.Is(err, ErrTraceNotFound) {
		t.Errorf("unknown trace error = %v, want ErrTraceNotFound", err)
	}
}

func TestMapAmbiguousTraceRequiresSelection(t *testing.T) {
	data := twoTraceExport(t)
	m := NewMapper()

	if _, err := m.MapBytes(data, Options{}); !errors.Is(err, ErrAmbiguousTrace) {
		t.Fatalf("error = %v, want ErrAmbiguousTrace", err)
	}

	run, err := m.MapBytes(data, Options{TraceID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"})
	if err != nil {
		t.Fatalf("selecting a trace failed: %v", err)
	}
	if len(run.Trace) != 1 || run.Trace[0].Name != "cancel_order" {
		t.Errorf("selected wrong trace: %+v", run.Trace)
	}

	var payload ExportPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	ids := TraceIDs(payload)
	if len(ids) != 2 || ids[0] != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Errorf("TraceIDs = %v, want sorted pair", ids)
	}
}

func TestErrorSpanMapsToFailedOutcome(t *testing.T) {
	export := []byte(`{
      "resourceSpans": [{
        "resource": {"attributes": [{"key": "service.name", "value": {"stringValue": "a"}}]},
        "scopeSpans": [{"spans": [{
          "traceId": "cccccccccccccccccccccccccccccccc",
          "spanId": "1111111111111111",
          "name": "AgentExecutor",
          "startTimeUnixNano": "1767225600000000000",
          "endTimeUnixNano": "1767225600010000000",
          "attributes": [
            {"key": "openinference.span.kind", "value": {"stringValue": "AGENT"}},
            {"key": "input.value", "value": {"stringValue": "do the thing"}}
          ],
          "status": {"code": "STATUS_CODE_ERROR", "message": "tool exploded"}
        }]}]
      }]
    }`)

	run, err := NewMapper().MapBytes(export, Options{})
	if err != nil {
		t.Fatalf("MapBytes failed: %v", err)
	}
	if run.Outcome.Status != "failed" || run.Outcome.Error != "tool exploded" {
		t.Errorf("outcome = %+v, want failed with error message", run.Outcome)
	}
	if run.Trace[0].Status.Code != "error" {
		t.Errorf("span status = %q, want error", run.Trace[0].Status.Code)
	}
	// Missing service.version must not produce a run that fails validation.
	if run.Agent.Version == "" {
		t.Error("agent version must fall back to a non-empty placeholder")
	}
}

func TestOptionsOverrideDerivedFields(t *testing.T) {
	run, err := NewMapper().MapBytes(goldenExport(t), Options{
		RunID:        "run-42",
		AgentName:    "override-agent",
		AgentVersion: "9.9",
		TaskID:       "task-42",
		TaskInput:    "custom input",
	})
	if err != nil {
		t.Fatalf("MapBytes failed: %v", err)
	}
	if run.RunID != "run-42" || run.Agent.Name != "override-agent" ||
		run.Agent.Version != "9.9" || run.Task.ID != "task-42" || run.Task.Input != "custom input" {
		t.Errorf("overrides not applied: %+v %+v %+v", run.RunID, run.Agent, run.Task)
	}
}

func twoTraceExport(t *testing.T) []byte {
	t.Helper()
	return []byte(`{
      "resourceSpans": [{
        "resource": {"attributes": [{"key": "service.name", "value": {"stringValue": "multi"}}]},
        "scopeSpans": [{"spans": [
          {
            "traceId": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
            "spanId": "1111111111111111",
            "name": "get_orders",
            "startTimeUnixNano": "1767225600000000000",
            "endTimeUnixNano": "1767225600010000000",
            "attributes": [
              {"key": "openinference.span.kind", "value": {"stringValue": "TOOL"}},
              {"key": "tool.name", "value": {"stringValue": "get_orders"}},
              {"key": "input.value", "value": {"stringValue": "list orders"}}
            ],
            "status": {"code": 1}
          },
          {
            "traceId": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
            "spanId": "2222222222222222",
            "name": "cancel_order",
            "startTimeUnixNano": "1767225600000000000",
            "endTimeUnixNano": "1767225600010000000",
            "attributes": [
              {"key": "openinference.span.kind", "value": {"stringValue": "TOOL"}},
              {"key": "tool.name", "value": {"stringValue": "cancel_order"}},
              {"key": "input.value", "value": {"stringValue": "cancel it"}}
            ],
            "status": {"code": 1}
          }
        ]}]
      }]
    }`)
}
