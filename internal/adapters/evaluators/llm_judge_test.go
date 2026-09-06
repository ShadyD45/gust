package evaluators_test

import (
	"context"
	"testing"

	"gust/internal/adapters/evaluators"
	"gust/internal/adapters/judge"
	"gust/internal/ports"
	"gust/pkg/api"
)

func sampleJudgeRun(status, output string) api.AgentRun {
	return api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "judge_test",
		Agent:         api.AgentInfo{Name: "a", Version: "1"},
		Task:          api.TaskInfo{ID: "t", Input: "summarize the order"},
		Trace:         nil,
		Outcome:       api.RunOutcome{Status: status, Output: output},
	}
}

func TestLLMJudgeDisabledByDefault(t *testing.T) {
	eval := evaluators.NewLLMJudgeEvaluator(&judge.MockProvider{})
	assert := &api.Assertion{
		ID:   "j1",
		Type: api.AssertLLMJudge,
		Parameters: map[string]any{
			"rubric": "Answer should mention the order was cancelled",
		},
	}
	res, err := eval.Evaluate(context.Background(), sampleJudgeRun("completed", "Order cancelled"), assert, ports.EvaluationContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Passed {
		t.Fatalf("expected fail-closed when allow_llm_judge is unset")
	}
	if res.Evidence["allow_llm_judge"] != false {
		t.Fatalf("evidence should record allow_llm_judge=false, got %#v", res.Evidence)
	}
}

func TestLLMJudgeWithMockProvider(t *testing.T) {
	eval := evaluators.NewLLMJudgeEvaluator(&judge.MockProvider{})
	assert := &api.Assertion{
		ID:   "j1",
		Type: api.AssertLLMJudge,
		Parameters: map[string]any{
			"rubric":    "Helpful cancellation confirmation",
			"threshold": 0.7,
		},
	}
	ctx := ports.EvaluationContext{Config: map[string]any{"allow_llm_judge": true}}
	res, err := eval.Evaluate(context.Background(), sampleJudgeRun("completed", "Order cancelled successfully with confirmation."), assert, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected pass, got %+v", res)
	}
	if res.Evidence["criticality"] != "soft" {
		t.Fatalf("expected soft criticality evidence, got %#v", res.Evidence)
	}
}

func TestLLMJudgeRequiresRubric(t *testing.T) {
	eval := evaluators.NewLLMJudgeEvaluator(&judge.MockProvider{})
	assert := &api.Assertion{ID: "j1", Type: api.AssertLLMJudge}
	ctx := ports.EvaluationContext{Config: map[string]any{"allow_llm_judge": true}}
	res, err := eval.Evaluate(context.Background(), sampleJudgeRun("completed", "ok"), assert, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Passed {
		t.Fatalf("expected failure without rubric")
	}
}

func TestValidateLLMJudgeAssertion(t *testing.T) {
	a := api.Assertion{ID: "j", Type: api.AssertLLMJudge, Criticality: api.CriticalitySoft}
	if err := api.ValidateAssertion(&a, 0); err != nil {
		t.Fatalf("ValidateAssertion: %v", err)
	}
}
