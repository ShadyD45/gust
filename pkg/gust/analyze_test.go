package gust_test

import (
	"context"
	"testing"
	"time"

	"gust/pkg/api"
	"gust/pkg/gust"
)

func TestAnalyze_BuiltinToolCall(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "run-1",
		Agent:         api.AgentInfo{Name: "demo", Version: "1"},
		Task:          api.TaskInfo{ID: "t1", Input: "cancel"},
		Trace: []api.Span{{
			SpanID:    "s1",
			Name:      "cancel_order",
			Type:      api.SpanTypeTool,
			StartTime: now,
			EndTime:   now,
			Attributes: map[string]any{
				"input":  map[string]any{"order_id": 123},
				"output": map[string]any{"ok": true},
			},
			Status: api.SpanStatus{Code: "ok"},
		}},
		Outcome: api.RunOutcome{Status: "completed", Output: "cancelled"},
	}
	assertions := []api.Assertion{
		{ID: "a1", Type: api.AssertTaskSuccess},
		{ID: "a2", Type: api.AssertToolCall, Tool: "cancel_order", Arguments: map[string]any{"order_id": 123}},
	}

	report, err := gust.Analyze(context.Background(), run, assertions, gust.WithScenarioID(run.RunID))
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed {
		t.Fatalf("expected pass, got %+v", report.Results)
	}
}

type alwaysFail struct{}

func (alwaysFail) Name() string    { return "always_fail" }
func (alwaysFail) Version() string { return "1.0.0" }
func (alwaysFail) Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx gust.EvaluationContext) (api.EvaluationResult, error) {
	return api.EvaluationResult{
		EvaluatorName:    "always_fail",
		EvaluatorVersion: "1.0.0",
		Passed:           false,
		Score:            0,
		Message:          "forced failure",
	}, nil
}

func TestAnalyze_ExtraEvaluator(t *testing.T) {
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "run-2",
		Agent:         api.AgentInfo{Name: "demo", Version: "1"},
		Task:          api.TaskInfo{ID: "t1", Input: "x"},
		Trace:         nil,
		Outcome:       api.RunOutcome{Status: "completed", Output: "ok"},
	}
	assertions := []api.Assertion{
		{ID: "custom", Type: "always_fail"},
	}
	report, err := gust.Analyze(context.Background(), run, assertions, gust.WithExtraEvaluators(alwaysFail{}))
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed {
		t.Fatal("expected custom evaluator to fail the report")
	}
}
