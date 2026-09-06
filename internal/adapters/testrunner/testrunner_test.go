package testrunner

import (
	"context"
	"testing"
	"time"

	"gust/pkg/api"
)

func TestSyntheticRunner_Probability(t *testing.T) {
	r := NewSyntheticRunner(1.0, 1)
	sc := api.TestScenario{
		ID:   "s1",
		Task: api.TaskInfo{ID: "t", Input: "x"},
		Reliability: api.ReliabilityConfig{
			Samples:         5,
			MinimumPassRate: 0.95,
			Confidence:      0.95,
		},
		Provenance: api.TestScenarioProvenance{Source: "test", ExtractedAt: time.Now().UTC()},
	}
	run, err := r.Run(context.Background(), sc, "")
	if err != nil {
		t.Fatal(err)
	}
	if run.Outcome.Status != "completed" {
		t.Fatalf("expected completed, got %s", run.Outcome.Status)
	}
}

func TestSyntheticRunner_FixedOutcomes(t *testing.T) {
	r := NewSyntheticRunner(0, 1).WithFixedOutcomes([]bool{true, false, true})
	sc := api.TestScenario{
		ID:   "s2",
		Task: api.TaskInfo{ID: "t", Input: "x"},
		Reliability: api.ReliabilityConfig{
			Samples:         3,
			MinimumPassRate: 0.95,
			Confidence:      0.95,
		},
		Provenance: api.TestScenarioProvenance{Source: "test", ExtractedAt: time.Now().UTC()},
	}
	var statuses []string
	for i := 0; i < 3; i++ {
		run, err := r.Run(context.Background(), sc, "")
		if err != nil {
			t.Fatal(err)
		}
		statuses = append(statuses, run.Outcome.Status)
	}
	want := []string{"completed", "failed", "completed"}
	for i := range want {
		if statuses[i] != want[i] {
			t.Fatalf("outcome %d: want %s got %s", i, want[i], statuses[i])
		}
	}
}
