package mutate

import (
	"context"
	"testing"
	"time"

	"gust/internal/adapters/evaluators"
	"gust/internal/adapters/mutators"
	"gust/internal/ports"
	"gust/pkg/api"
)

func buildGoldenSuite() []GoldenCase {
	now := time.Now().UTC()

	run1 := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "golden_order_cancel",
		Agent:         api.AgentInfo{Name: "support", Version: "1.0"},
		Task:          api.TaskInfo{ID: "t1", Input: "cancel order 123"},
		Trace: []api.Span{
			{
				SpanID:    "s1",
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
				SpanID:    "s2",
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
			Output: "Order 123 cancelled successfully.",
		},
	}

	assertions1 := []api.Assertion{
		{ID: "a1", Type: api.AssertTaskSuccess, Parameters: map[string]any{"expected_output": "cancelled"}},
		{ID: "a2", Type: api.AssertToolCall, Tool: "get_orders", Arguments: map[string]any{"customer_id": 42}},
		{ID: "a3", Type: api.AssertToolCall, Tool: "cancel_order", Arguments: map[string]any{"order_id": 123}},
		{ID: "a4", Type: api.AssertForbiddenToolCall, Tool: "forbidden_admin_access"},
		{ID: "a5", Type: api.AssertMaxSteps, Limit: 2}, // Tight limit detects duplicate calls
	}

	// Case 2: Recovery case testing error recovery
	run2 := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "golden_recovery",
		Agent:         api.AgentInfo{Name: "support", Version: "1.0"},
		Task:          api.TaskInfo{ID: "t2", Input: "fetch doc"},
		Trace: []api.Span{
			{
				SpanID:    "s2_1",
				Name:      "fetch_doc",
				Type:      api.SpanTypeError,
				StartTime: now,
				EndTime:   now.Add(10 * time.Millisecond),
				Status:    api.SpanStatus{Code: "error", Message: "network glitch"},
			},
			{
				SpanID:    "s2_2",
				Name:      "fetch_doc",
				Type:      api.SpanTypeTool,
				StartTime: now.Add(20 * time.Millisecond),
				EndTime:   now.Add(30 * time.Millisecond),
				Status:    api.SpanStatus{Code: "ok"},
			},
		},
		Outcome: api.RunOutcome{
			Status: "completed",
			Output: "recovered doc",
		},
	}

	assertions2 := []api.Assertion{
		{ID: "b1", Type: api.AssertErrorRecovery},
		{ID: "b2", Type: api.AssertTaskSuccess, Parameters: map[string]any{"expected_output": "recovered doc"}},
		{ID: "b3", Type: api.AssertMaxSteps, Limit: 2},
		{ID: "b4", Type: api.AssertToolCall, Tool: "fetch_doc"},
	}

	return []GoldenCase{
		{Run: run1, Assertions: assertions1},
		{Run: run2, Assertions: assertions2},
	}
}

func TestMutationBenchmarkTargetMetrics(t *testing.T) {
	evalList := []ports.Evaluator{
		&evaluators.TaskSuccessEvaluator{},
		&evaluators.ToolSelectionEvaluator{},
		&evaluators.ToolArgumentsEvaluator{},
		&evaluators.ToolSequenceEvaluator{},
		&evaluators.ForbiddenToolEvaluator{},
		&evaluators.RequiredToolEvaluator{},
		&evaluators.MaxStepsEvaluator{},
		&evaluators.MaxLatencyEvaluator{},
		&evaluators.ErrorRecoveryEvaluator{},
		&evaluators.SchemaValidationEvaluator{},
	}

	mutList := mutators.AllBuiltinMutators()
	runner := NewRunner(mutList, evalList)
	goldenSuite := buildGoldenSuite()

	report, err := runner.RunBenchmark(context.Background(), goldenSuite)
	if err != nil {
		t.Fatalf("RunBenchmark failed: %v", err)
	}

	t.Logf("Generated %d mutants, %d applied, %d detected", report.TotalMutantsGenerated, report.MutantsApplied, report.MutantsDetected)
	t.Logf("Detection Rate: %.2f%%", report.DetectionRate*100)
	t.Logf("False Positive Rate: %.2f%%", report.FalsePositiveRate*100)

	for name, stats := range report.ClassBreakdown {
		t.Logf("  [%s]: %d/%d detected (%.1f%%)", name, stats.Detected, stats.Applied, stats.Rate*100)
	}

	// Flagship Trust Metrics Assertions
	if report.FalsePositiveRate > 0.05 {
		t.Errorf("False Positive Rate exceeded 5%% threshold: %.2f%%", report.FalsePositiveRate*100)
	}

	if report.DetectionRate < 0.90 {
		t.Errorf("Detection Rate below 90%% target: %.2f%%", report.DetectionRate*100)
	}
}
