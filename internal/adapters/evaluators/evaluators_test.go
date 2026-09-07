package evaluators

import (
	"context"
	"strings"
	"testing"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

func sampleRun() api.AgentRun {
	now := time.Now().UTC()
	return api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "run_eval_test",
		Agent:         api.AgentInfo{Name: "agent", Version: "1.0"},
		Task:          api.TaskInfo{ID: "t1", Input: "Cancel my order"},
		Trace: []api.Span{
			{
				SpanID:    "span_1",
				Name:      "get_orders",
				Type:      api.SpanTypeTool,
				StartTime: now,
				EndTime:   now.Add(10 * time.Millisecond),
				Attributes: map[string]any{
					"input": map[string]any{"customer_id": 42},
				},
				Status: api.SpanStatus{Code: "ok"},
			},
			{
				SpanID:    "span_2",
				Name:      "cancel_order",
				Type:      api.SpanTypeTool,
				StartTime: now.Add(15 * time.Millisecond),
				EndTime:   now.Add(25 * time.Millisecond),
				Attributes: map[string]any{
					"input": map[string]any{"order_id": 123},
				},
				Status: api.SpanStatus{Code: "ok"},
			},
		},
		Outcome: api.RunOutcome{
			Status: "completed",
			Output: "Order 123 cancelled.",
		},
	}
}

func TestTaskSuccessEvaluator(t *testing.T) {
	eval := &TaskSuccessEvaluator{}
	ctx := context.Background()

	run := sampleRun()
	res, err := eval.Evaluate(ctx, run, nil, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("expected pass, got %v (res: %+v)", err, res)
	}

	// Test failure on failed outcome
	run.Outcome.Status = "failed"
	res, err = eval.Evaluate(ctx, run, nil, ports.EvaluationContext{})
	if err != nil || res.Passed {
		t.Fatalf("expected fail on failed outcome status, got: %+v", res)
	}
}

func TestToolSelectionEvaluator(t *testing.T) {
	eval := &ToolSelectionEvaluator{}
	ctx := context.Background()
	run := sampleRun()

	// Should pass for cancel_order
	assertPass := &api.Assertion{Type: api.AssertToolCall, Tool: "cancel_order"}
	res, err := eval.Evaluate(ctx, run, assertPass, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("expected pass for cancel_order: %+v", res)
	}

	// Should fail for uncalled tool
	assertFail := &api.Assertion{Type: api.AssertToolCall, Tool: "uncalled_tool"}
	res, err = eval.Evaluate(ctx, run, assertFail, ports.EvaluationContext{})
	if err != nil || res.Passed {
		t.Fatalf("expected fail for uncalled_tool: %+v", res)
	}
}

func TestToolArgumentsEvaluator(t *testing.T) {
	eval := &ToolArgumentsEvaluator{}
	ctx := context.Background()
	run := sampleRun()

	// Matching arguments
	assertPass := &api.Assertion{
		Type:      api.AssertToolCall,
		Tool:      "cancel_order",
		Arguments: map[string]any{"order_id": 123},
	}
	res, err := eval.Evaluate(ctx, run, assertPass, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("expected pass for matching arguments: %+v", res)
	}

	// Mismatched arguments
	assertFail := &api.Assertion{
		Type:      api.AssertToolCall,
		Tool:      "cancel_order",
		Arguments: map[string]any{"order_id": 999},
	}
	res, err = eval.Evaluate(ctx, run, assertFail, ports.EvaluationContext{})
	if err != nil || res.Passed {
		t.Fatalf("expected fail for mismatched arguments: %+v", res)
	}
}

func TestToolArgumentsOccurrence(t *testing.T) {
	eval := &ToolArgumentsEvaluator{}
	ctx := context.Background()
	now := time.Now().UTC()
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "run_occ",
		Agent:         api.AgentInfo{Name: "agent", Version: "1.0"},
		Task:          api.TaskInfo{ID: "t1", Input: "cancel"},
		Trace: []api.Span{
			{
				SpanID: "s1", Name: "cancel_order", Type: api.SpanTypeTool,
				StartTime: now, EndTime: now.Add(time.Millisecond),
				Attributes: map[string]any{"input": map[string]any{"order_id": 999}},
				Status:     api.SpanStatus{Code: "ok"},
			},
			{
				SpanID: "s2", Name: "cancel_order", Type: api.SpanTypeTool,
				StartTime: now.Add(2 * time.Millisecond), EndTime: now.Add(3 * time.Millisecond),
				Attributes: map[string]any{"input": map[string]any{"order_id": 123}},
				Status:     api.SpanStatus{Code: "ok"},
			},
		},
		Outcome: api.RunOutcome{Status: "completed"},
	}

	anyAssert := &api.Assertion{
		Type: api.AssertToolCall, Tool: "cancel_order",
		Arguments:  map[string]any{"order_id": 123},
		Parameters: map[string]any{"occurrence": "any"},
	}
	res, err := eval.Evaluate(ctx, run, anyAssert, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("expected any to pass: %+v", res)
	}

	firstAssert := &api.Assertion{
		Type: api.AssertToolCall, Tool: "cancel_order",
		Arguments:  map[string]any{"order_id": 123},
		Parameters: map[string]any{"occurrence": "first"},
	}
	res, err = eval.Evaluate(ctx, run, firstAssert, ports.EvaluationContext{})
	if err != nil || res.Passed {
		t.Fatalf("expected first to fail: %+v", res)
	}

	lastAssert := &api.Assertion{
		Type: api.AssertToolCall, Tool: "cancel_order",
		Arguments:  map[string]any{"order_id": 123},
		Parameters: map[string]any{"occurrence": "last"},
	}
	res, err = eval.Evaluate(ctx, run, lastAssert, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("expected last to pass: %+v", res)
	}

	nthAssert := &api.Assertion{
		Type: api.AssertToolCall, Tool: "cancel_order",
		Arguments:  map[string]any{"order_id": 123},
		Parameters: map[string]any{"occurrence": 2},
	}
	res, err = eval.Evaluate(ctx, run, nthAssert, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("expected occurrence 2 to pass: %+v", res)
	}
}

func TestToolSequenceEvaluator(t *testing.T) {
	eval := &ToolSequenceEvaluator{}
	ctx := context.Background()
	run := sampleRun()

	// Correct sequence: get_orders then cancel_order
	assertPass := &api.Assertion{
		Type: api.AssertToolSequence,
		Parameters: map[string]any{
			"sequence": []any{"get_orders", "cancel_order"},
		},
	}
	res, err := eval.Evaluate(ctx, run, assertPass, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("expected sequence to pass: %+v", res)
	}

	// Broken sequence: cancel_order then get_orders
	assertFail := &api.Assertion{
		Type: api.AssertToolSequence,
		Parameters: map[string]any{
			"sequence": []any{"cancel_order", "get_orders"},
		},
	}
	res, err = eval.Evaluate(ctx, run, assertFail, ports.EvaluationContext{})
	if err != nil || res.Passed {
		t.Fatalf("expected broken sequence to fail: %+v", res)
	}
}

func TestToolSequenceMatchModes(t *testing.T) {
	eval := &ToolSequenceEvaluator{}
	ctx := context.Background()
	now := time.Now().UTC()
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "run_seq",
		Agent:         api.AgentInfo{Name: "agent", Version: "1.0"},
		Task:          api.TaskInfo{ID: "t1", Input: "task"},
		Trace: []api.Span{
			{SpanID: "1", Name: "get_orders", Type: api.SpanTypeTool, StartTime: now, EndTime: now.Add(time.Millisecond), Status: api.SpanStatus{Code: "ok"}},
			{SpanID: "2", Name: "lookup", Type: api.SpanTypeTool, StartTime: now.Add(2 * time.Millisecond), EndTime: now.Add(3 * time.Millisecond), Status: api.SpanStatus{Code: "ok"}},
			{SpanID: "3", Name: "cancel_order", Type: api.SpanTypeTool, StartTime: now.Add(4 * time.Millisecond), EndTime: now.Add(5 * time.Millisecond), Status: api.SpanStatus{Code: "ok"}},
		},
		Outcome: api.RunOutcome{Status: "completed"},
	}

	sub := &api.Assertion{
		Type: api.AssertToolSequence,
		Parameters: map[string]any{
			"sequence": []any{"get_orders", "cancel_order"},
			"match":    "subsequence",
		},
	}
	res, err := eval.Evaluate(ctx, run, sub, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("subsequence should pass with extras: %+v", res)
	}

	exact := &api.Assertion{
		Type: api.AssertToolSequence,
		Parameters: map[string]any{
			"sequence": []any{"get_orders", "cancel_order"},
			"match":    "exact",
		},
	}
	res, err = eval.Evaluate(ctx, run, exact, ports.EvaluationContext{})
	if err != nil || res.Passed {
		t.Fatalf("exact should fail with extras: %+v", res)
	}

	exactOK := &api.Assertion{
		Type: api.AssertToolSequence,
		Parameters: map[string]any{
			"sequence": []any{"get_orders", "lookup", "cancel_order"},
			"match":    "exact",
		},
	}
	res, err = eval.Evaluate(ctx, run, exactOK, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("exact should pass when identical: %+v", res)
	}
}

func TestForbiddenToolEvaluator(t *testing.T) {
	eval := &ForbiddenToolEvaluator{}
	ctx := context.Background()
	run := sampleRun()

	// Tool not in trace -> pass
	assertPass := &api.Assertion{Type: api.AssertForbiddenToolCall, Tool: "delete_db"}
	res, err := eval.Evaluate(ctx, run, assertPass, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("expected pass for uncalled forbidden tool: %+v", res)
	}

	// Tool is in trace -> fail
	assertFail := &api.Assertion{Type: api.AssertForbiddenToolCall, Tool: "cancel_order"}
	res, err = eval.Evaluate(ctx, run, assertFail, ports.EvaluationContext{})
	if err != nil || res.Passed {
		t.Fatalf("expected fail for called forbidden tool: %+v", res)
	}
}

func TestRequiredToolEvaluator(t *testing.T) {
	eval := &RequiredToolEvaluator{}
	ctx := context.Background()
	run := sampleRun()

	assertPass := &api.Assertion{Type: api.AssertRequiredTool, Tool: "get_orders"}
	res, err := eval.Evaluate(ctx, run, assertPass, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("expected pass for required tool: %+v", res)
	}

	assertFail := &api.Assertion{Type: api.AssertRequiredTool, Tool: "refund_money"}
	res, err = eval.Evaluate(ctx, run, assertFail, ports.EvaluationContext{})
	if err != nil || res.Passed {
		t.Fatalf("expected fail for missing required tool: %+v", res)
	}
}

func TestMaxStepsEvaluator(t *testing.T) {
	eval := &MaxStepsEvaluator{}
	ctx := context.Background()
	run := sampleRun() // 2 tool spans

	assertPass := &api.Assertion{Type: api.AssertMaxSteps, Limit: 5}
	res, err := eval.Evaluate(ctx, run, assertPass, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("expected pass for step limit 5: %+v", res)
	}

	assertFail := &api.Assertion{Type: api.AssertMaxSteps, Limit: 1}
	res, err = eval.Evaluate(ctx, run, assertFail, ports.EvaluationContext{})
	if err != nil || res.Passed {
		t.Fatalf("expected fail for step limit 1: %+v", res)
	}

	// LLM spans do not count toward max_steps.
	now := time.Now().UTC()
	mixed := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "run_steps",
		Agent:         api.AgentInfo{Name: "agent", Version: "1.0"},
		Task:          api.TaskInfo{ID: "t1", Input: "x"},
		Trace: []api.Span{
			{SpanID: "1", Name: "think", Type: api.SpanTypeLLM, StartTime: now, EndTime: now, Status: api.SpanStatus{Code: "ok"}},
			{SpanID: "2", Name: "tool_a", Type: api.SpanTypeTool, StartTime: now, EndTime: now, Status: api.SpanStatus{Code: "ok"}},
			{SpanID: "3", Name: "retrieve", Type: api.SpanTypeRetrieval, StartTime: now, EndTime: now, Status: api.SpanStatus{Code: "ok"}},
		},
		Outcome: api.RunOutcome{Status: "completed", Output: "ok"},
	}
	res, err = eval.Evaluate(ctx, mixed, &api.Assertion{Type: api.AssertMaxSteps, Limit: 1}, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("expected pass counting only tool/agent spans: %+v", res)
	}
}

func TestSchemaValidationEvaluator(t *testing.T) {
	eval := &SchemaValidationEvaluator{}
	ctx := context.Background()
	run := sampleRun()

	// Envelope-only fallback when no schema is declared.
	res, err := eval.Evaluate(ctx, run, nil, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("expected envelope pass: %+v", res)
	}
	if !strings.Contains(res.Message, "envelope") {
		t.Fatalf("expected envelope-only message, got %q", res.Message)
	}
	if res.Evidence["envelope_only"] != true {
		t.Fatalf("expected envelope_only evidence, got %#v", res.Evidence)
	}

	schemaAssert := &api.Assertion{
		Type: api.AssertSchemaValid,
		Parameters: map[string]any{
			"schema": map[string]any{
				"type":     "object",
				"required": []any{"status", "order_id"},
				"properties": map[string]any{
					"status":   map[string]any{"type": "string"},
					"order_id": map[string]any{"type": "integer"},
				},
			},
		},
	}

	passRun := run
	passRun.Outcome.Output = `{"status":"cancelled","order_id":123}`
	res, err = eval.Evaluate(ctx, passRun, schemaAssert, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("expected schema pass: %+v", res)
	}
	if res.Message != "output satisfies declared schema" {
		t.Fatalf("expected real schema pass message, got %q", res.Message)
	}
	if res.Evidence["envelope_only"] != false {
		t.Fatalf("pass path must not be envelope-only, got %#v", res.Evidence)
	}

	failRun := run
	failRun.Outcome.Output = `{"status":"cancelled"}`
	res, err = eval.Evaluate(ctx, failRun, schemaAssert, ports.EvaluationContext{})
	if err != nil || res.Passed {
		t.Fatalf("expected schema fail for missing required field: %+v", res)
	}
	if !strings.Contains(res.Message, "schema validation") {
		t.Fatalf("expected schema validation failure message, got %q", res.Message)
	}
	if _, ok := res.Evidence["validation_error"]; !ok {
		t.Fatalf("expected validation_error evidence, got %#v", res.Evidence)
	}

	typeFail := run
	typeFail.Outcome.Output = `{"status":"cancelled","order_id":"not-an-int"}`
	res, err = eval.Evaluate(ctx, typeFail, schemaAssert, ports.EvaluationContext{})
	if err != nil || res.Passed {
		t.Fatalf("expected schema fail for wrong type: %+v", res)
	}

	badJSON := run
	badJSON.Outcome.Output = `not-json`
	res, err = eval.Evaluate(ctx, badJSON, schemaAssert, ports.EvaluationContext{})
	if err != nil || res.Passed {
		t.Fatalf("expected non-JSON fail: %+v", res)
	}

	// Valid AgentRun envelope must still fail if output violates the declared schema
	// (guards against regressing to envelope-only no-op when schema is present).
	envelopeOK := run
	envelopeOK.Outcome.Output = `{"wrong":true}`
	if err := envelopeOK.Validate(); err != nil {
		t.Fatalf("precondition: envelope should validate: %v", err)
	}
	res, err = eval.Evaluate(ctx, envelopeOK, schemaAssert, ports.EvaluationContext{})
	if err != nil || res.Passed {
		t.Fatalf("must not no-op to envelope pass when schema is declared: %+v", res)
	}
}

func TestMaxLatencyEvaluator(t *testing.T) {
	eval := &MaxLatencyEvaluator{}
	ctx := context.Background()
	run := sampleRun() // ~25ms duration

	assertPass := &api.Assertion{Type: api.AssertMaxLatency, Limit: 1000} // 1s
	res, err := eval.Evaluate(ctx, run, assertPass, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("expected pass for latency limit 1000ms: %+v", res)
	}
}

func TestMaxLatencyUnorderedSpans(t *testing.T) {
	eval := &MaxLatencyEvaluator{}
	ctx := context.Background()
	now := time.Now().UTC()
	// Slice order is reverse of wall-clock order; old logic would get negative/wrong duration.
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "run_lat",
		Agent:         api.AgentInfo{Name: "agent", Version: "1.0"},
		Task:          api.TaskInfo{ID: "t1", Input: "task"},
		Trace: []api.Span{
			{
				SpanID: "late", Name: "b", Type: api.SpanTypeTool,
				StartTime: now.Add(50 * time.Millisecond), EndTime: now.Add(100 * time.Millisecond),
				Status: api.SpanStatus{Code: "ok"},
			},
			{
				SpanID: "early", Name: "a", Type: api.SpanTypeTool,
				StartTime: now, EndTime: now.Add(10 * time.Millisecond),
				Status: api.SpanStatus{Code: "ok"},
			},
		},
		Outcome: api.RunOutcome{Status: "completed"},
	}

	res, err := eval.Evaluate(ctx, run, &api.Assertion{Type: api.AssertMaxLatency, Limit: 200}, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("expected pass within 200ms wall clock: %+v", res)
	}
	actualMs, ok := res.Evidence["actual_latency_ms"].(int64)
	if !ok {
		if v, ok2 := res.Evidence["actual_latency_ms"].(int); ok2 {
			actualMs = int64(v)
			ok = true
		}
	}
	if !ok || actualMs != 100 {
		if !strings.Contains(res.Message, "100") {
			t.Fatalf("expected ~100ms wall latency, got evidence=%v message=%q", res.Evidence, res.Message)
		}
	}

	res, err = eval.Evaluate(ctx, run, &api.Assertion{Type: api.AssertMaxLatency, Limit: 50}, ports.EvaluationContext{})
	if err != nil || res.Passed {
		t.Fatalf("expected fail for 50ms limit on 100ms span: %+v", res)
	}
}

func TestErrorRecoveryEvaluator(t *testing.T) {
	eval := &ErrorRecoveryEvaluator{}
	ctx := context.Background()

	now := time.Now().UTC()
	recoveredRun := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "run_rec",
		Agent:         api.AgentInfo{Name: "agent", Version: "1.0"},
		Task:          api.TaskInfo{ID: "t1", Input: "task"},
		Trace: []api.Span{
			{
				SpanID:    "span_err",
				Name:      "api_call",
				Type:      api.SpanTypeTool,
				StartTime: now,
				EndTime:   now.Add(10 * time.Millisecond),
				Status:    api.SpanStatus{Code: "error", Message: "HTTP 500"},
			},
			{
				SpanID:    "span_retry",
				Name:      "api_call",
				Type:      api.SpanTypeTool,
				StartTime: now.Add(20 * time.Millisecond),
				EndTime:   now.Add(30 * time.Millisecond),
				Status:    api.SpanStatus{Code: "ok"},
			},
		},
		Outcome: api.RunOutcome{Status: "completed"},
	}

	res, err := eval.Evaluate(ctx, recoveredRun, nil, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("expected pass for recovered run: %+v", res)
	}
}

func TestErrorRecoveryRejectsUnrelatedSuccess(t *testing.T) {
	eval := &ErrorRecoveryEvaluator{}
	ctx := context.Background()
	now := time.Now().UTC()
	// Review false-positive: get_customer errors, then unrelated weather succeeds.
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "run_false_rec",
		Agent:         api.AgentInfo{Name: "agent", Version: "1.0"},
		Task:          api.TaskInfo{ID: "t1", Input: "task"},
		Trace: []api.Span{
			{
				SpanID: "e1", Name: "get_customer", Type: api.SpanTypeTool,
				StartTime: now, EndTime: now.Add(time.Millisecond),
				Status: api.SpanStatus{Code: "error"},
			},
			{
				SpanID: "w1", Name: "retrieve_weather", Type: api.SpanTypeTool,
				StartTime: now.Add(2 * time.Millisecond), EndTime: now.Add(3 * time.Millisecond),
				Status: api.SpanStatus{Code: "ok"},
			},
			{
				SpanID: "a1", Name: "final_answer", Type: api.SpanTypeAgent,
				StartTime: now.Add(4 * time.Millisecond), EndTime: now.Add(5 * time.Millisecond),
				Status: api.SpanStatus{Code: "ok"},
			},
		},
		Outcome: api.RunOutcome{Status: "completed"},
	}

	res, err := eval.Evaluate(ctx, run, nil, ports.EvaluationContext{})
	if err != nil || res.Passed {
		t.Fatalf("expected fail when recovery is unrelated tool: %+v", res)
	}

	// Explicit recovery_tools can allow a different successful tool.
	okAssert := &api.Assertion{
		Type: api.AssertErrorRecovery,
		Parameters: map[string]any{
			"after_error_tool": "get_customer",
			"recovery_tools":   []any{"retrieve_weather"},
		},
	}
	res, err = eval.Evaluate(ctx, run, okAssert, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("expected pass with explicit recovery_tools: %+v", res)
	}
}

// Benchmark Throughput Target: >= 1,000 cases/sec per core
func BenchmarkEvaluatorsThroughput(b *testing.B) {
	eval := &TaskSuccessEvaluator{}
	run := sampleRun()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = eval.Evaluate(ctx, run, nil, ports.EvaluationContext{})
	}
}

// TestEvaluatorThroughputGate enforces the MVP DoD of ≥1,000 evaluations/sec.
func TestEvaluatorThroughputGate(t *testing.T) {
	evals := AllBuiltinEvaluators()
	run := sampleRun()
	ctx := context.Background()
	const n = 5000

	start := time.Now()
	for i := 0; i < n; i++ {
		e := evals[i%len(evals)]
		if _, err := e.Evaluate(ctx, run, &api.Assertion{ID: "a", Type: api.AssertTaskSuccess}, ports.EvaluationContext{}); err != nil {
			t.Fatalf("evaluate: %v", err)
		}
	}
	elapsed := time.Since(start)
	rate := float64(n) / elapsed.Seconds()
	t.Logf("throughput: %.0f evals/sec over %d evaluations", rate, n)
	if rate < 1000 {
		t.Fatalf("throughput %.0f evals/sec below DoD gate of 1000/sec", rate)
	}
}
