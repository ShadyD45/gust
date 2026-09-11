package scenario

import (
	"testing"

	"gust/pkg/api"
)

func TestProposeFromRunLeavesAssertionsEmptyAndRedacts(t *testing.T) {
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "prod-1",
		Agent:         api.AgentInfo{Name: "agent", Version: "1"},
		Task:          api.TaskInfo{ID: "t", Input: "email alice@example.com"},
		Trace: []api.Span{{
			SpanID: "s1", Name: "lookup", Type: api.SpanTypeTool,
			Attributes: map[string]any{
				"input":  map[string]any{"email": "alice@example.com"},
				"output": "ok",
			},
			Status: api.SpanStatus{Code: "ok"},
		}},
		Outcome: api.RunOutcome{Status: "completed"},
	}
	p, err := NewExtractor().ProposeFromRun(run, ProposeOptions{Redact: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Scenario.Assertions) != 0 {
		t.Fatalf("assertions must stay empty, got %d", len(p.Scenario.Assertions))
	}
	if p.Scenario.Provenance.ReviewedBy != "" {
		t.Fatal("reviewed_by must be empty until human review")
	}
	if p.Scenario.Provenance.SourceRunID != "prod-1" {
		t.Fatalf("source run id: %q", p.Scenario.Provenance.SourceRunID)
	}
	if p.Redaction.TotalReplaced == 0 {
		t.Fatalf("expected redaction hits: %+v", p.Redaction)
	}
}

func TestProposeRequiresOptOutNote(t *testing.T) {
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "r",
		Agent:         api.AgentInfo{Name: "a", Version: "1"},
		Task:          api.TaskInfo{ID: "t", Input: "x"},
		Outcome:       api.RunOutcome{Status: "completed"},
	}
	_, err := NewExtractor().ProposeFromRun(run, ProposeOptions{Redact: false})
	if err == nil {
		t.Fatal("expected error without opt-out note")
	}
}
