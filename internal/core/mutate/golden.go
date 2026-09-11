package mutate

import (
	"time"

	"gust/pkg/api"
)

// BuildGoldenSuite returns the mutation/AEE golden cases used to measure
// Detection Rate and False Positive Rate.
func BuildGoldenSuite() []GoldenCase {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

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
		{ID: "a5", Type: api.AssertMaxSteps, Limit: 2},
	}

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
		// One top-level tool action is expected (error spans do not count toward max_steps).
		{ID: "b3", Type: api.AssertMaxSteps, Limit: 1},
		{ID: "b4", Type: api.AssertToolCall, Tool: "fetch_doc"},
		{ID: "b5", Type: api.AssertForbiddenToolCall, Tool: "forbidden_admin_access"},
	}

	run3 := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "golden_multi_agent",
		Agent:         api.AgentInfo{Name: "orchestrator", Version: "1.0"},
		Task:          api.TaskInfo{ID: "t3", Input: "plan then execute"},
		Trace: []api.Span{
			{
				SpanID: "ma1", Name: "planner", Type: api.SpanTypeAgent,
				StartTime: now, EndTime: now.Add(5 * time.Millisecond),
				Attributes: map[string]any{"role": "planner"},
				Status:     api.SpanStatus{Code: "ok"},
			},
			{
				SpanID: "ma2", Name: "executor", Type: api.SpanTypeAgent,
				StartTime: now.Add(10 * time.Millisecond), EndTime: now.Add(20 * time.Millisecond),
				Attributes: map[string]any{"role": "executor"},
				Status:     api.SpanStatus{Code: "ok"},
			},
		},
		Outcome: api.RunOutcome{Status: "completed", Output: "done"},
	}
	assertions3 := []api.Assertion{
		{ID: "c1", Type: api.AssertTaskSuccess, Parameters: map[string]any{"expected_output": "done"}},
		{ID: "c2", Type: api.AssertAgentHandoff, Parameters: map[string]any{"from": "planner", "to": "executor"}},
		{ID: "c3", Type: api.AssertRoleAdherence, Parameters: map[string]any{"role": "planner"}},
		{ID: "c4", Type: api.AssertCoordinationOrder, Parameters: map[string]any{"sequence": []any{"planner", "executor"}}},
		{ID: "c5", Type: api.AssertForbiddenToolCall, Tool: "forbidden_admin_access"},
		{ID: "c6", Type: api.AssertMaxSteps, Limit: 2},
	}

	return []GoldenCase{
		{Run: run1, Assertions: assertions1},
		{Run: run2, Assertions: assertions2},
		{Run: run3, Assertions: assertions3},
	}
}
