package scenario

import (
	"strings"
	"testing"
	"time"

	"gust/pkg/api"
)

func TestExtractFromRun_H8EmptyAssertions(t *testing.T) {
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "run_buggy_18291",
		Agent:         api.AgentInfo{Name: "support", Version: "1.0"},
		Task:          api.TaskInfo{ID: "refund-001", Input: "Cancel my latest order"},
		Outcome:       api.RunOutcome{Status: "completed"},
		Trace: []api.Span{
			{
				SpanID: "s1",
				Name:   "cancel_order",
				Type:   api.SpanTypeTool,
				Attributes: map[string]any{
					"input":  map[string]any{"order_id": 122}, // buggy: wrong order
					"output": map[string]any{"ok": true},
				},
				StartTime: time.Now().UTC(),
				EndTime:   time.Now().UTC(),
				Status:    api.SpanStatus{Code: "ok"},
			},
		},
	}

	sc, err := NewExtractor().ExtractFromRun(run)
	if err != nil {
		t.Fatalf("ExtractFromRun: %v", err)
	}
	if len(sc.Assertions) != 0 {
		t.Fatalf("H8 violated: assertions must be empty, got %+v", sc.Assertions)
	}
	if len(sc.Environment.Fixtures) != 1 {
		t.Fatalf("expected 1 fixture, got %d", len(sc.Environment.Fixtures))
	}
	fx := sc.Environment.Fixtures[0]
	if fx.Tool != "cancel_order" || fx.Provenance != api.ProvenanceRecorded {
		t.Fatalf("unexpected fixture: %+v", fx)
	}
	if fx.InputHash == "" {
		t.Fatal("expected content hash on fixture")
	}
	// Must not enshrine buggy cancel as an assertion expectation
	for _, a := range sc.Assertions {
		if a.Tool == "cancel_order" {
			t.Fatal("buggy cancel_order must not appear as assertion")
		}
	}
}

func TestRenderYAML_ContainsWarning(t *testing.T) {
	sc := &api.TestScenario{
		ID:          "scenario_x",
		Version:     "1.0",
		Description: "TODO",
		Task:        api.TaskInfo{ID: "t", Input: "hi"},
		Assertions:  []api.Assertion{},
		Reliability: api.ReliabilityConfig{Samples: 10, MinimumPassRate: 0.95, Confidence: 0.95},
		Provenance:  api.TestScenarioProvenance{Source: "production_trace", ExtractedAt: time.Now().UTC()},
	}
	raw, err := RenderYAML(sc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Trace is not the test") {
		t.Fatalf("missing H8 warning in yaml:\n%s", raw)
	}
}
