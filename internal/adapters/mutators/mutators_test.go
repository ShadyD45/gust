package mutators

import (
	"context"
	"testing"
	"time"

	"gust/pkg/api"
)

func TestRemoveRequiredToolMutator(t *testing.T) {
	now := time.Now().UTC()
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "r1",
		Agent:         api.AgentInfo{Name: "a", Version: "1"},
		Task:          api.TaskInfo{ID: "t", Input: "x"},
		Trace: []api.Span{{
			SpanID: "s1", Name: "tool_a", Type: api.SpanTypeTool,
			StartTime: now, EndTime: now, Status: api.SpanStatus{Code: "ok"},
		}},
		Outcome: api.RunOutcome{Status: "completed"},
	}
	out, err := (&RemoveRequiredToolMutator{}).Mutate(context.Background(), run)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != api.MutationApplied {
		t.Fatalf("expected applied, got %s", out.Status)
	}
	if len(out.MutatedRun.Trace) != 0 {
		t.Fatalf("expected empty trace, got %d", len(out.MutatedRun.Trace))
	}
}
