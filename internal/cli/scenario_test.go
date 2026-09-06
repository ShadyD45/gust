package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gust/pkg/api"
)

func TestWriteScenarioDir_Loadable(t *testing.T) {
	sc := &api.TestScenario{
		ID:          "scenario_from_run",
		Version:     "1.0",
		Description: "TODO",
		Task:        api.TaskInfo{ID: "t", Input: "Cancel my latest order"},
		Environment: api.EnvironmentSpec{
			FixtureStrategy: api.MatchStrategyHybrid,
			Fixtures: []api.Fixture{{
				FixtureID:     "fx_cancel_order_001",
				Tool:          "cancel_order",
				MatchStrategy: api.MatchStrategyExactHash,
				RecordedInput: map[string]any{"order_id": 123},
				RecordedResponse: api.RecordedResponse{
					Status: "success",
					Body:   map[string]any{"ok": true},
				},
				Provenance: api.ProvenanceRecorded,
			}},
		},
		Assertions:  []api.Assertion{},
		Reliability: api.ReliabilityConfig{Samples: 10, MinimumPassRate: 0.95, Confidence: 0.95},
		Provenance:  api.TestScenarioProvenance{Source: "production_trace", ExtractedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	dir := t.TempDir()
	if err := writeScenarioDir(dir, sc); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "scenario.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "fixtures", "fx_cancel_order_001.json")); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadScenario(filepath.Join(dir, "scenario.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Assertions) != 0 {
		t.Fatalf("assertions must stay empty, got %d", len(loaded.Assertions))
	}
	if len(loaded.Environment.Fixtures) != 1 || loaded.Environment.Fixtures[0].FixtureID != "fx_cancel_order_001" {
		t.Fatalf("fixtures %+v", loaded.Environment.Fixtures)
	}
}
