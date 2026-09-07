package evaluators_test

import (
	"context"
	"testing"

	"gust/internal/adapters/evaluators"
	"gust/internal/core/analyze"
	"gust/internal/ports"
	"gust/pkg/api"
)

type stubJudge struct {
	name   string
	passed bool
	score  float64
	err    error
}

func (s stubJudge) Name() string    { return s.name }
func (s stubJudge) Version() string { return "1" }
func (s stubJudge) Evaluate(context.Context, api.AgentRun, *api.Assertion, ports.EvaluationContext) (ports.EvaluationResult, error) {
	if s.err != nil {
		return ports.EvaluationResult{}, s.err
	}
	return ports.EvaluationResult{
		EvaluatorName: s.name,
		Passed:        s.passed,
		Score:         s.score,
		Message:       s.name,
		Evidence:      map[string]any{"model": s.name, "rationale": s.name},
	}, nil
}

func panelCtx(judges ...ports.Evaluator) ports.EvaluationContext {
	peers := map[string]ports.Evaluator{}
	for _, j := range judges {
		peers[j.Name()] = j
	}
	return ports.EvaluationContext{Config: map[string]any{
		"allow_llm_judge": true,
		"peer_evaluators": peers,
	}}
}

func TestJudgePanelMajority(t *testing.T) {
	panel := &evaluators.JudgePanelEvaluator{}
	assert := &api.Assertion{
		ID:   "p",
		Type: api.AssertJudgePanel,
		Parameters: map[string]any{
			"judges":      []any{"a", "b", "c"},
			"aggregation": "majority",
			"rubric":      "be helpful",
		},
	}
	res, err := panel.Evaluate(context.Background(), api.AgentRun{}, assert, panelCtx(
		stubJudge{name: "a", passed: true, score: 0.9},
		stubJudge{name: "b", passed: true, score: 0.8},
		stubJudge{name: "c", passed: false, score: 0.2},
	))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Passed {
		t.Fatalf("majority should pass: %+v", res)
	}
	if res.Evidence["disagreement"] != true {
		t.Fatalf("expected disagreement, got %#v", res.Evidence)
	}
}

func TestJudgePanelAllAndAny(t *testing.T) {
	panel := &evaluators.JudgePanelEvaluator{}
	ctx := panelCtx(
		stubJudge{name: "a", passed: true, score: 0.9},
		stubJudge{name: "b", passed: false, score: 0.2},
	)
	all := &api.Assertion{ID: "p", Type: api.AssertJudgePanel, Parameters: map[string]any{
		"judges": []any{"a", "b"}, "aggregation": "all", "rubric": "x",
	}}
	res, err := panel.Evaluate(context.Background(), api.AgentRun{}, all, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Passed {
		t.Fatal("all should fail")
	}
	anyA := &api.Assertion{ID: "p", Type: api.AssertJudgePanel, Parameters: map[string]any{
		"judges": []any{"a", "b"}, "aggregation": "any", "rubric": "x",
	}}
	res, err = panel.Evaluate(context.Background(), api.AgentRun{}, anyA, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Passed {
		t.Fatal("any should pass")
	}
}

func TestJudgePanelMeanScore(t *testing.T) {
	panel := &evaluators.JudgePanelEvaluator{}
	assert := &api.Assertion{ID: "p", Type: api.AssertJudgePanel, Parameters: map[string]any{
		"judges": []any{"a", "b"}, "aggregation": "mean_score", "threshold": 0.7, "rubric": "x",
	}}
	res, err := panel.Evaluate(context.Background(), api.AgentRun{}, assert, panelCtx(
		stubJudge{name: "a", passed: true, score: 0.9},
		stubJudge{name: "b", passed: false, score: 0.6},
	))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Passed {
		t.Fatalf("mean 0.75 should pass: %+v", res)
	}
}

func TestJudgePanelRejectsEmptyUnknownDuplicateNested(t *testing.T) {
	panel := &evaluators.JudgePanelEvaluator{}
	ctx := panelCtx(stubJudge{name: "a", passed: true, score: 1})
	cases := []map[string]any{
		{"judges": []any{}, "rubric": "x"},
		{"judges": []any{"missing"}, "rubric": "x"},
		{"judges": []any{"a", "a"}, "rubric": "x"},
		{"judges": []any{"judge_panel"}, "rubric": "x"},
	}
	for i, params := range cases {
		res, err := panel.Evaluate(context.Background(), api.AgentRun{}, &api.Assertion{
			ID: "p", Type: api.AssertJudgePanel, Parameters: params,
		}, ctx)
		if err != nil {
			t.Fatalf("case %d err %v", i, err)
		}
		if res.Passed {
			t.Fatalf("case %d should fail: %+v", i, res)
		}
	}
}

func TestJudgePanelAnalyzeSoftUntilCalibrated(t *testing.T) {
	engine := analyze.NewEngine([]ports.Evaluator{
		stubJudge{name: "a", passed: false, score: 0.1},
		&evaluators.JudgePanelEvaluator{},
	})
	assert := api.Assertion{
		ID:          "p",
		Type:        api.AssertJudgePanel,
		Criticality: api.CriticalityHard,
		Parameters: map[string]any{
			"judges": []any{"a"}, "aggregation": "all", "rubric": "x",
		},
	}
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "r",
		Agent:         api.AgentInfo{Name: "a", Version: "1"},
		Task:          api.TaskInfo{ID: "t", Input: "x"},
		Outcome:       api.RunOutcome{Status: "completed"},
	}
	report, err := engine.AnalyzeRun(context.Background(), run, []api.Assertion{assert}, ports.EvaluationContext{
		Config: map[string]any{"allow_llm_judge": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed {
		t.Fatal("uncalibrated panel must stay soft")
	}
	report, err = engine.AnalyzeRun(context.Background(), run, []api.Assertion{assert}, ports.EvaluationContext{
		Config: map[string]any{"allow_llm_judge": true, "llm_judge_calibrated": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed {
		t.Fatal("calibrated hard panel should fail the sample")
	}
}
