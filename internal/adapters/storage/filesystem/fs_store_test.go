package filesystem

import (
	"context"
	"os"
	"testing"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

func TestFileStoreScenarios(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "store_test_scenarios_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}

	ctx := context.Background()
	sc := api.TestScenario{
		ID:          "test_sc_1",
		Version:     "1.0",
		Description: "A test scenario",
		Task: api.TaskInfo{
			Input: "Execute action",
		},
		Reliability: api.ReliabilityConfig{
			Samples:         10,
			MinimumPassRate: 0.95,
			Confidence:      0.95,
		},
		Provenance: api.TestScenarioProvenance{
			Source:      "unit_test",
			ExtractedAt: time.Now().UTC(),
		},
	}

	if err := store.SaveScenario(ctx, sc); err != nil {
		t.Fatalf("SaveScenario failed: %v", err)
	}

	fetched, err := store.GetScenario(ctx, "test_sc_1")
	if err != nil {
		t.Fatalf("GetScenario failed: %v", err)
	}
	if fetched.ID != sc.ID || fetched.Task.Input != sc.Task.Input {
		t.Errorf("fetched scenario does not match: %+v", fetched)
	}

	list, err := store.ListScenarios(ctx)
	if err != nil {
		t.Fatalf("ListScenarios failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 scenario in list, got %d", len(list))
	}

	if err := store.DeleteScenario(ctx, "test_sc_1"); err != nil {
		t.Fatalf("DeleteScenario failed: %v", err)
	}

	_, err = store.GetScenario(ctx, "test_sc_1")
	if err != ports.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestFileStoreFixtures(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "store_test_fixtures_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}

	ctx := context.Background()
	fx := api.Fixture{
		FixtureID: "fx_101",
		Tool:      "search_kb",
		InputHash: "sha256:1111111111111111111111111111111111111111111111111111111111111111",
		RecordedResponse: api.RecordedResponse{
			Status: "success",
			Body:   "doc content",
		},
		Provenance: api.ProvenanceRecorded,
	}

	if err := store.SaveFixture(ctx, fx); err != nil {
		t.Fatalf("SaveFixture failed: %v", err)
	}

	found, ok, err := store.FindByHash(ctx, "search_kb", fx.InputHash)
	if err != nil || !ok {
		t.Fatalf("FindByHash failed: ok=%v, err=%v", ok, err)
	}
	if found.FixtureID != "fx_101" {
		t.Errorf("found fixture id mismatch: %s", found.FixtureID)
	}
}

func TestFileStoreRuns(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "store_test_runs_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}

	ctx := context.Background()
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "run_store_test",
		Agent:         api.AgentInfo{Name: "agent_a", Version: "1.0"},
		Task:          api.TaskInfo{ID: "t_1", Input: "task a"},
		Outcome:       api.RunOutcome{Status: "completed"},
	}

	if err := store.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun failed: %v", err)
	}

	fetched, err := store.GetRun(ctx, "run_store_test")
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}
	if fetched.RunID != run.RunID {
		t.Errorf("fetched run ID mismatch: %s", fetched.RunID)
	}
}
