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
	m, err := Bundle(dir, "ds1", []api.TestScenario{sc})
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
