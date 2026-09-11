package cluster

import (
	"testing"

	"gust/pkg/api"
)

func baseRun(id, tool string, errCode string) api.AgentRun {
	span := api.Span{
		SpanID: "t1", Name: tool, Type: api.SpanTypeTool,
		Attributes: map[string]any{"input": map[string]any{"order_id": "x"}},
		Status:     api.SpanStatus{Code: "ok"},
	}
	if errCode != "" {
		span.Status = api.SpanStatus{Code: "error", Message: errCode}
	}
	status := "completed"
	if errCode != "" {
		status = "failed"
	}
	return api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         id,
		Agent:         api.AgentInfo{Name: "a", Version: "1"},
		Task:          api.TaskInfo{ID: "t", Input: "x"},
		Trace:         []api.Span{span},
		Outcome:       api.RunOutcome{Status: status, Error: errCode},
	}
}

func TestClusterCollapsesDuplicates(t *testing.T) {
	runs := []api.AgentRun{
		baseRun("a", "cancel_order", "not_found"),
		baseRun("b", "cancel_order", "not_found"), // duplicate mode
		baseRun("c", "refund", "timeout"),         // distinct
	}
	rep, err := ClusterRuns(runs)
	if err != nil {
		t.Fatal(err)
	}
	if rep.TotalRuns != 3 {
		t.Fatalf("total=%d", rep.TotalRuns)
	}
	if len(rep.Clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d: %+v", len(rep.Clusters), rep.Clusters)
	}
	reps := Representatives(runs, rep)
	if len(reps) != 2 {
		t.Fatalf("representatives=%d", len(reps))
	}
}

func TestDistinctArgShapesSeparate(t *testing.T) {
	a := baseRun("a", "lookup", "")
	b := baseRun("b", "lookup", "")
	b.Trace[0].Attributes["input"] = map[string]any{"order_id": "x", "extra": true}
	rep, err := ClusterRuns([]api.AgentRun{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Clusters) != 2 {
		t.Fatalf("different arg schemas should separate, got %d", len(rep.Clusters))
	}
}
