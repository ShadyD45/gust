package testrunner

import (
	"testing"

	"gust/internal/ports"
	"gust/pkg/api"
)

func TestNormalizeSampleRequestPreservesWorldMode(t *testing.T) {
	req := ports.SampleRequest{
		Scenario: api.TestScenario{
			ID:   "s",
			Task: api.TaskInfo{ID: "t", Input: "x"},
		},
		FixtureEndpoint: "http://fixtures",
		WorldMode:       api.WorldControlGust,
	}
	got := normalizeSampleRequest(req, "")
	if got.WorldMode != api.WorldControlGust {
		t.Fatalf("world mode %s", got.WorldMode)
	}
	if got.FixtureEndpoint != "http://fixtures" {
		t.Fatalf("fixture endpoint %s", got.FixtureEndpoint)
	}
	if got.SampleID == "" || got.TraceID == "" || got.TraceParent == "" {
		t.Fatalf("expected identity fields, got %+v", got)
	}
}

func TestAttainableHelpersWired(t *testing.T) {
	req := NewSampleRequest("eval", api.TestScenario{ID: "s", Task: api.TaskInfo{ID: "t", Input: "x"}}, "", "")
	if req.EvaluationID != "eval" || req.WorldMode != api.WorldControlExisting {
		t.Fatalf("%+v", req)
	}
}
