package evaluators

import (
	"context"
	"testing"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

func multiAgentRun() api.AgentRun {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "ma1",
		Agent:         api.AgentInfo{Name: "orchestrator", Version: "1"},
		Task:          api.TaskInfo{ID: "t", Input: "plan"},
		Trace: []api.Span{
			{SpanID: "a1", Name: "planner", Type: api.SpanTypeAgent, StartTime: now, EndTime: now.Add(time.Millisecond), Status: api.SpanStatus{Code: "ok"}, Attributes: map[string]any{"role": "planner"}},
			{SpanID: "a2", Name: "executor", Type: api.SpanTypeAgent, StartTime: now.Add(2 * time.Millisecond), EndTime: now.Add(3 * time.Millisecond), Status: api.SpanStatus{Code: "ok"}, Attributes: map[string]any{"role": "executor"}},
		},
		Outcome: api.RunOutcome{Status: "completed"},
	}
}

func TestMultiAgentEvaluators(t *testing.T) {
	run := multiAgentRun()
	ctx := context.Background()

	h, err := (&AgentHandoffEvaluator{}).Evaluate(ctx, run, &api.Assertion{
		Type: api.AssertAgentHandoff,
		Parameters: map[string]any{"from": "planner", "to": "executor"},
	}, ports.EvaluationContext{})
	if err != nil || !h.Passed {
		t.Fatalf("handoff: %+v err=%v", h, err)
	}

	r, err := (&RoleAdherenceEvaluator{}).Evaluate(ctx, run, &api.Assertion{
		Type: api.AssertRoleAdherence, Parameters: map[string]any{"role": "executor"},
	}, ports.EvaluationContext{})
	if err != nil || !r.Passed {
		t.Fatalf("role: %+v err=%v", r, err)
	}

	c, err := (&CoordinationOrderEvaluator{}).Evaluate(ctx, run, &api.Assertion{
		Type: api.AssertCoordinationOrder,
		Parameters: map[string]any{"sequence": []any{"planner", "executor"}},
	}, ports.EvaluationContext{})
	if err != nil || !c.Passed {
		t.Fatalf("coordination: %+v err=%v", c, err)
	}
}
