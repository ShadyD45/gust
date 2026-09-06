package evaluators

import (
	"context"
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
	run := sampleRun() // 2 spans

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

func TestSchemaValidationEvaluator(t *testing.T) {
	eval := &SchemaValidationEvaluator{}
	ctx := context.Background()
	run := sampleRun()

	res, err := eval.Evaluate(ctx, run, nil, ports.EvaluationContext{})
	if err != nil || !res.Passed {
		t.Fatalf("expected pass for valid schema: %+v", res)
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
