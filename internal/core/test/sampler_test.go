package test

import (
	"context"
	"testing"
	"time"

	"gust/internal/adapters/evaluators"
	"gust/internal/adapters/testrunner"
	"gust/internal/ports"
	"gust/pkg/api"
)

func baseScenario(id string, samples int) api.TestScenario {
	return api.TestScenario{
		ID:          id,
		Version:     "1.0",
		Description: "H7 synthetic reliability",
		Task:        api.TaskInfo{ID: "t1", Input: "complete the task"},
		Assertions: []api.Assertion{{
			ID:   "assert_success",
			Type: api.AssertTaskSuccess,
		}},
		Reliability: api.ReliabilityConfig{
			Samples:         samples,
			MinimumPassRate: 0.95,
			Confidence:      0.95,
		},
		Provenance: api.TestScenarioProvenance{
			Source:      "unit_test",
			ExtractedAt: time.Now().UTC(),
		},
	}
}

func builtinEvals() []ports.Evaluator {
	return []ports.Evaluator{&evaluators.TaskSuccessEvaluator{}}
}

func TestH7_PassAtN100(t *testing.T) {
	runner := testrunner.NewSyntheticRunner(1.0, 42)
	sampler := NewSampler(builtinEvals())
	res, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario:    baseScenario("h7_pass", 100),
		Runner:      runner,
		Evaluators:  builtinEvals(),
		Concurrency: 8,
	})
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if res.Verdict != api.VerdictPass {
		t.Fatalf("expected PASS for 100/100, got %s (rate=%.3f ci=[%.3f,%.3f])",
			res.Verdict, res.ObservedPassRate, res.ConfidenceInterval[0], res.ConfidenceInterval[1])
	}
}

func TestH7_FlakyAtN20Perfect(t *testing.T) {
	runner := testrunner.NewSyntheticRunner(1.0, 7)
	sampler := NewSampler(builtinEvals())
	res, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario:    baseScenario("h7_flaky", 20),
		Runner:      runner,
		Concurrency: 4,
	})
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if res.Verdict != api.VerdictFlaky {
		t.Fatalf("expected FLAKY for 20/20 vs 0.95, got %s (ci=[%.3f,%.3f])",
			res.Verdict, res.ConfidenceInterval[0], res.ConfidenceInterval[1])
	}
}

func TestH7_FailAtLowRate(t *testing.T) {
	runner := testrunner.NewSyntheticRunner(0.0, 99)
	sampler := NewSampler(builtinEvals())
	res, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario:    baseScenario("h7_fail", 20),
		Runner:      runner,
		Concurrency: 4,
	})
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if res.Verdict != api.VerdictFail {
		t.Fatalf("expected FAIL for 0/20, got %s", res.Verdict)
	}
}

func TestH7_FailSeventeenOfTwenty(t *testing.T) {
	outcomes := make([]bool, 20)
	for i := 0; i < 17; i++ {
		outcomes[i] = true
	}
	runner := testrunner.NewSyntheticRunner(0, 1).WithFixedOutcomes(outcomes)
	sampler := NewSampler(builtinEvals())
	res, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario: baseScenario("h7_fail_17", 20),
		Runner:   runner,
	})
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if res.Passes != 17 {
		t.Fatalf("expected 17 passes, got %d", res.Passes)
	}
	if res.Verdict != api.VerdictFail {
		t.Fatalf("expected FAIL for 17/20, got %s (ci=[%.3f,%.3f])",
			res.Verdict, res.ConfidenceInterval[0], res.ConfidenceInterval[1])
	}
}
