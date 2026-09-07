package dataset

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gust/pkg/api"
)

func TestBundleAndVerify(t *testing.T) {
	dir := t.TempDir()
	sc := api.TestScenario{
		ID:          "sc_a",
		Version:     "1.0",
		Description: "d",
		Task:        api.TaskInfo{ID: "t", Input: "in"},
		Reliability: api.ReliabilityConfig{Samples: 5, MinimumPassRate: 0.9, Confidence: 0.95},
		Provenance:  api.TestScenarioProvenance{Source: "test", ExtractedAt: time.Now().UTC()},
	}
	m, err := Bundle(dir, "ds1", []api.TestScenario{sc}, false)
	if err != nil {
		t.Fatal(err)
	}
	if m.ContentHash == "" {
		t.Fatal("expected content hash")
	}
	if err := Verify(dir); err != nil {
		t.Fatalf("verify: %v", err)
	}

	sc.Description = "tampered"
	data, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sc_a.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := Verify(dir); err == nil {
		t.Fatal("expected checksum failure after tamper")
	}
}

func TestBundleRefusesSilentOverwrite(t *testing.T) {
	dir := t.TempDir()
	sc := api.TestScenario{
		ID:          "sc_a",
		Version:     "1.0",
		Description: "d",
		Task:        api.TaskInfo{ID: "t", Input: "in"},
		Reliability: api.ReliabilityConfig{Samples: 5, MinimumPassRate: 0.9, Confidence: 0.95},
		Provenance:  api.TestScenarioProvenance{Source: "test", ExtractedAt: time.Now().UTC()},
	}
	if _, err := Bundle(dir, "ds1", []api.TestScenario{sc}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := Bundle(dir, "ds1", []api.TestScenario{sc}, false); err != nil {
		t.Fatalf("identical re-bundle should be idempotent: %v", err)
	}
	sc.Description = "changed"
	if _, err := Bundle(dir, "ds1", []api.TestScenario{sc}, false); err == nil {
		t.Fatal("expected overwrite error without --force")
	}
	if _, err := Bundle(dir, "ds1", []api.TestScenario{sc}, true); err != nil {
		t.Fatalf("force overwrite: %v", err)
	}
}

func TestBundlePruneOrphansRequiresForce(t *testing.T) {
	dir := t.TempDir()
	a := api.TestScenario{
		ID: "sc_a", Version: "1.0", Task: api.TaskInfo{ID: "t", Input: "in"},
		Reliability: api.ReliabilityConfig{Samples: 5, MinimumPassRate: 0.9, Confidence: 0.95},
		Provenance:  api.TestScenarioProvenance{Source: "test", ExtractedAt: time.Now().UTC()},
	}
	b := a
	b.ID = "sc_b"
	if _, err := Bundle(dir, "ds1", []api.TestScenario{a, b}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := Bundle(dir, "ds1", []api.TestScenario{a}, false); err == nil {
		t.Fatal("expected orphan error without --force")
	}
	if _, err := Bundle(dir, "ds1", []api.TestScenario{a}, true); err != nil {
		t.Fatalf("force prune: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sc_b.json")); !os.IsNotExist(err) {
		t.Fatalf("orphan should be removed, stat err=%v", err)
	}
}
