package policy

import (
	"testing"

	"gust/pkg/api"
)

func flakyResult() *api.ReliabilityResult {
	return &api.ReliabilityResult{
		ScenarioID:         "sc_flaky",
		Samples:            20,
		Passes:             20,
		ObservedPassRate:   1.0,
		ConfidenceInterval: [2]float64{0.84, 1.0},
		Verdict:            api.VerdictFlaky,
	}
}

func TestOnFlakyWarnVsFail(t *testing.T) {
	eng := NewEngine()
	results := []*api.ReliabilityResult{flakyResult()}

	warnPol := api.Policy{
		Name: "warn",
		Reliability: api.PolicyReliability{
			DefaultMinimumPassRate: 0.95,
			MinSamplesForVerdict:   5,
			OnFlaky:                "warn",
		},
	}
	vWarn := eng.Evaluate(warnPol, results)
	if vWarn.ExitCode != api.ExitSuccess || vWarn.OverallVerdict != api.VerdictPass {
		t.Fatalf("warn: want exit 0 PASS, got exit=%d verdict=%s", vWarn.ExitCode, vWarn.OverallVerdict)
	}

	failPol := warnPol
	failPol.Name = "fail"
	failPol.Reliability.OnFlaky = "fail"
	vFail := eng.Evaluate(failPol, results)
	if vFail.ExitCode != api.ExitFlakyFailure || vFail.OverallVerdict != api.VerdictFlaky {
		t.Fatalf("fail: want exit 3 FLAKY, got exit=%d verdict=%s", vFail.ExitCode, vFail.OverallVerdict)
	}
}

func TestHardConstraintPriority(t *testing.T) {
	eng := NewEngine()
	res := &api.ReliabilityResult{
		ScenarioID:           "sc_hard",
		Samples:              20,
		Passes:               19,
		ObservedPassRate:     0.95,
		ConfidenceInterval:   [2]float64{0.76, 0.99},
		Verdict:              api.VerdictFlaky,
		HardConstraintFailed: true,
		PerRunEvidence: []api.EvaluationResult{{
			EvaluatorName: "forbidden_tool",
			Passed:        false,
		}},
	}
	pol := api.Policy{
		Name: "prod",
		Reliability: api.PolicyReliability{
			DefaultMinimumPassRate: 0.95,
			OnFlaky:                "warn",
		},
	}
	v := eng.Evaluate(pol, []*api.ReliabilityResult{res})
	if v.ExitCode != api.ExitFailure || v.OverallVerdict != api.VerdictFail {
		t.Fatalf("expected hard FAIL exit 1, got exit=%d verdict=%s", v.ExitCode, v.OverallVerdict)
	}
}

func TestRegressionSignificant(t *testing.T) {
	base := ExperimentStats{Name: "base", Passes: 95, Samples: 100, PassRate: 0.95, LatencyNs: 1e6}
	noise := ExperimentStats{Name: "cand", Passes: 94, Samples: 100, PassRate: 0.949, LatencyNs: 1e6}
	pol := api.PolicyRegression{MaxPassRateDrop: 0.02, MaxLatencyIncreaseRatio: 0.15}

	rNoise := CompareRegression(base, noise, pol)
	if rNoise.Regressed {
		t.Fatalf("tiny drop should not regress: %+v", rNoise)
	}

	bad := ExperimentStats{Name: "cand", Passes: 75, Samples: 100, PassRate: 0.75, LatencyNs: 1e6}
	rBad := CompareRegression(base, bad, pol)
	if !rBad.Regressed || !rBad.Significant {
		t.Fatalf("large drop should regress: %+v", rBad)
	}
}
