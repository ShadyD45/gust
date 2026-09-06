package analyze

import (
	"context"
	"testing"

	"gust/internal/adapters/evaluators"
	"gust/internal/ports"
	"gust/pkg/api"
)

func TestAnalyzeRun_TaskSuccess(t *testing.T) {
	engine := NewEngine([]ports.Evaluator{&evaluators.TaskSuccessEvaluator{}})
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "run_analyze_1",
		Agent:         api.AgentInfo{Name: "a", Version: "1"},
		Task:          api.TaskInfo{ID: "t", Input: "do it"},
		Outcome:       api.RunOutcome{Status: "completed"},
	}
	assertions := []api.Assertion{{
		ID:   "a1",
		Type: api.AssertTaskSuccess,
	}}

	report, err := engine.AnalyzeRun(context.Background(), run, assertions, ports.EvaluationContext{})
	if err != nil {
		t.Fatalf("AnalyzeRun: %v", err)
	}
	if !report.Passed {
		t.Fatalf("expected pass, got %+v", report.Results)
	}
}
