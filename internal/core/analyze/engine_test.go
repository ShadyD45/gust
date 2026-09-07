package analyze

import (
	"context"
	"testing"

	"gust/internal/adapters/evaluators"
	"gust/internal/adapters/judge"
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

func TestAnalyzeRun_SoftFailureDoesNotFailReport(t *testing.T) {
	engine := NewEngine([]ports.Evaluator{&evaluators.TaskSuccessEvaluator{}})
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "run_soft",
		Agent:         api.AgentInfo{Name: "a", Version: "1"},
		Task:          api.TaskInfo{ID: "t", Input: "do it"},
		Outcome:       api.RunOutcome{Status: "failed"},
	}
	report, err := engine.AnalyzeRun(context.Background(), run, []api.Assertion{{
		ID:          "soft_success",
		Type:        api.AssertTaskSuccess,
		Criticality: api.CriticalitySoft,
	}}, ports.EvaluationContext{})
	if err != nil {
		t.Fatalf("AnalyzeRun: %v", err)
	}
	if !report.Passed {
		t.Fatalf("soft failure should not fail the report, got %+v", report.Results)
	}
	if report.Results[0].Passed {
		t.Fatal("evaluator should still record a failed assertion")
	}
	if report.Results[0].Evidence["policy_hard"] == true {
		t.Fatal("soft assertion must not be a policy hard constraint")
	}
}

func TestAnalyzeRun_UncalibratedHardJudgeDoesNotFailReport(t *testing.T) {
	engine := NewEngine([]ports.Evaluator{evaluators.NewLLMJudgeEvaluator(&judge.MockProvider{})})
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "run_judge_soft",
		Agent:         api.AgentInfo{Name: "a", Version: "1"},
		Task:          api.TaskInfo{ID: "t", Input: "do it"},
		Outcome:       api.RunOutcome{Status: "completed", Output: "nope"},
	}
	report, err := engine.AnalyzeRun(context.Background(), run, []api.Assertion{{
		ID:          "tone",
		Type:        api.AssertLLMJudge,
		Criticality: api.CriticalityHard,
		Parameters:  map[string]any{"rubric": "Must mention cancellation", "threshold": 0.99},
	}}, ports.EvaluationContext{Config: map[string]any{"allow_llm_judge": true}})
	if err != nil {
		t.Fatalf("AnalyzeRun: %v", err)
	}
	if !report.Passed {
		t.Fatalf("uncalibrated judge must stay soft, got %+v", report.Results)
	}
	if report.Results[0].Criticality != api.CriticalitySoft {
		t.Fatalf("criticality=%s want soft", report.Results[0].Criticality)
	}
}

func TestAnalyzeRun_CalibratedHardJudgeFailsReport(t *testing.T) {
	engine := NewEngine([]ports.Evaluator{evaluators.NewLLMJudgeEvaluator(&judge.MockProvider{})})
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "run_judge_hard",
		Agent:         api.AgentInfo{Name: "a", Version: "1"},
		Task:          api.TaskInfo{ID: "t", Input: "do it"},
		Outcome:       api.RunOutcome{Status: "completed", Output: "nope"},
	}
	report, err := engine.AnalyzeRun(context.Background(), run, []api.Assertion{{
		ID:          "tone",
		Type:        api.AssertLLMJudge,
		Criticality: api.CriticalityHard,
		Parameters:  map[string]any{"rubric": "Must mention cancellation", "threshold": 0.99},
	}}, ports.EvaluationContext{Config: map[string]any{
		"allow_llm_judge":      true,
		"llm_judge_calibrated": true,
	}})
	if err != nil {
		t.Fatalf("AnalyzeRun: %v", err)
	}
	if report.Passed {
		t.Fatal("calibrated hard judge should fail the sample")
	}
	if report.Results[0].Criticality != api.CriticalityHard {
		t.Fatalf("criticality=%s want hard", report.Results[0].Criticality)
	}
}
