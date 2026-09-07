package policy

import (
	"strings"
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

func TestOnFlakyWarnVsFailVsIgnore(t *testing.T) {
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
	if vWarn.ExitCode != api.ExitSuccess || vWarn.OverallVerdict != api.VerdictFlaky {
		t.Fatalf("warn: want exit 0 FLAKY, got exit=%d verdict=%s", vWarn.ExitCode, vWarn.OverallVerdict)
	}

	ignorePol := warnPol
	ignorePol.Name = "ignore"
	ignorePol.Reliability.OnFlaky = "ignore"
	vIgnore := eng.Evaluate(ignorePol, results)
	if vIgnore.ExitCode != api.ExitSuccess || vIgnore.OverallVerdict != api.VerdictPass {
		t.Fatalf("ignore: want exit 0 PASS, got exit=%d verdict=%s", vIgnore.ExitCode, vIgnore.OverallVerdict)
	}

	failPol := warnPol
	failPol.Name = "fail"
	failPol.Reliability.OnFlaky = "fail"
	vFail := eng.Evaluate(failPol, results)
	if vFail.ExitCode != api.ExitFlakyFailure || vFail.OverallVerdict != api.VerdictFlaky {
		t.Fatalf("fail: want exit 3 FLAKY, got exit=%d verdict=%s", vFail.ExitCode, vFail.OverallVerdict)
	}
}

func TestHardConstraintProcessesAllScenarios(t *testing.T) {
	eng := NewEngine()
	results := []*api.ReliabilityResult{
		{
			ScenarioID:           "sc_hard",
			Samples:              20,
			Passes:               19,
			Verdict:              api.VerdictFlaky,
			HardConstraintFailed: true,
			PerRunEvidence: []api.EvaluationResult{{
				EvaluatorName: "forbidden_tool",
				Passed:        false,
			}},
		},
		{
			ScenarioID:       "sc_pass",
			Samples:          100,
			Passes:           100,
			ObservedPassRate: 1.0,
			Verdict:          api.VerdictPass,
		},
		{
			ScenarioID:       "sc_fail",
			Samples:          20,
			Passes:           0,
			ObservedPassRate: 0,
			Verdict:          api.VerdictFail,
		},
	}
	pol := api.Policy{
		Name: "prod",
		Reliability: api.PolicyReliability{
			DefaultMinimumPassRate: 0.95,
			OnFlaky:                "warn",
		},
	}
	v := eng.Evaluate(pol, results)
	if v.ExitCode != api.ExitFailure || v.OverallVerdict != api.VerdictFail {
		t.Fatalf("expected hard FAIL exit 1, got exit=%d verdict=%s", v.ExitCode, v.OverallVerdict)
	}
	if len(v.ScenarioResults) != 3 {
		t.Fatalf("expected all 3 scenario results, got %d", len(v.ScenarioResults))
	}
	for _, id := range []string{"sc_hard", "sc_pass", "sc_fail"} {
		if _, ok := v.ScenarioResults[id]; !ok {
			t.Errorf("missing scenario result %s", id)
		}
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

func TestRegressionZeroBaselineLatency(t *testing.T) {
	base := ExperimentStats{Name: "base", Passes: 95, Samples: 100, PassRate: 0.95, LatencyNs: 0}
	cand := ExperimentStats{Name: "cand", Passes: 95, Samples: 100, PassRate: 0.95, LatencyNs: 1e9}
	pol := api.PolicyRegression{MaxPassRateDrop: 0.02, MaxLatencyIncreaseRatio: 0.15}
	r := CompareRegression(base, cand, pol)
	if r.LatencyComparable {
		t.Fatalf("zero baseline latency should not be comparable")
	}
	if r.LatencyIncreaseRatio != nil {
		t.Fatalf("unmeasurable latency ratio should be omitted, got %v", *r.LatencyIncreaseRatio)
	}
	if r.Regressed {
		t.Fatalf("should not regress on unmeasurable latency: %+v", r)
	}
	if r.Message == "no significant regression" {
		t.Fatalf("message should note latency was not measurable, got %q", r.Message)
	}
}

func TestHardConstraintThresholdAllowsSomeFailures(t *testing.T) {
	eng := NewEngine()
	res := &api.ReliabilityResult{
		ScenarioID: "sc_tol",
		Samples:    10,
		Passes:     9,
		Verdict:    api.VerdictPass,
		PerRunEvidence: []api.EvaluationResult{
			{EvaluatorName: "forbidden_tool", Passed: false},
		},
	}
	pol := api.Policy{
		Name:            "tol",
		HardConstraints: api.HardConstraints{ForbiddenTools: 1, SchemaViolations: 0},
		Reliability:     api.PolicyReliability{DefaultMinimumPassRate: 0.95, OnFlaky: "warn"},
	}
	v := eng.Evaluate(pol, []*api.ReliabilityResult{res})
	if v.ExitCode != api.ExitSuccess {
		t.Fatalf("one forbidden failure should be allowed when max is 1, got exit=%d", v.ExitCode)
	}

	res.PerRunEvidence = append(res.PerRunEvidence, api.EvaluationResult{EvaluatorName: "forbidden_tool", Passed: false})
	v = eng.Evaluate(pol, []*api.ReliabilityResult{res})
	if v.ExitCode != api.ExitFailure {
		t.Fatalf("two forbidden failures should exceed max 1, got exit=%d", v.ExitCode)
	}
	if len(v.Violations) == 0 || !strings.Contains(v.Violations[0].Message, "forbidden_tools 2 > 1") {
		t.Fatalf("expected named-count violation, got %#v", v.Violations)
	}
}

func TestHardConstraintSchemaThreshold(t *testing.T) {
	eng := NewEngine()
	res := &api.ReliabilityResult{
		ScenarioID: "sc_schema",
		Samples:    10,
		Passes:     10,
		Verdict:    api.VerdictPass,
		PerRunEvidence: []api.EvaluationResult{
			{EvaluatorName: "schema_validation", Passed: false},
		},
	}
	allow := api.Policy{
		Name:            "allow_one",
		HardConstraints: api.HardConstraints{ForbiddenTools: 0, SchemaViolations: 1},
		Reliability:     api.PolicyReliability{DefaultMinimumPassRate: 0.95, OnFlaky: "warn"},
	}
	v := eng.Evaluate(allow, []*api.ReliabilityResult{res})
	if v.ExitCode != api.ExitSuccess {
		t.Fatalf("one schema failure should be allowed when max is 1, got exit=%d", v.ExitCode)
	}

	deny := allow
	deny.HardConstraints.SchemaViolations = 0
	v = eng.Evaluate(deny, []*api.ReliabilityResult{res})
	if v.ExitCode != api.ExitFailure {
		t.Fatalf("one schema failure should exceed max 0, got exit=%d", v.ExitCode)
	}
	if len(v.Violations) == 0 || !strings.Contains(v.Violations[0].Message, "schema_violations 1 > 0") {
		t.Fatalf("expected schema count violation, got %#v", v.Violations)
	}
}

func TestHardConstraintBooleanCannotOverrideYAML(t *testing.T) {
	eng := NewEngine()
	res := &api.ReliabilityResult{
		ScenarioID:           "sc_flag",
		Samples:              10,
		Passes:               9,
		Verdict:              api.VerdictPass,
		HardConstraintFailed: true,
		PerRunEvidence: []api.EvaluationResult{
			{EvaluatorName: "forbidden_tool", Passed: false},
		},
	}
	pol := api.Policy{
		Name:            "tol",
		HardConstraints: api.HardConstraints{ForbiddenTools: 1, SchemaViolations: 0},
		Reliability:     api.PolicyReliability{DefaultMinimumPassRate: 0.95, OnFlaky: "warn"},
	}
	v := eng.Evaluate(pol, []*api.ReliabilityResult{res})
	if v.ExitCode != api.ExitSuccess {
		t.Fatalf("HardConstraintFailed must not override YAML max 1, got exit=%d", v.ExitCode)
	}

	res.PerRunEvidence = nil
	v = eng.Evaluate(pol, []*api.ReliabilityResult{res})
	if v.ExitCode != api.ExitSuccess {
		t.Fatalf("stale HardConstraintFailed with no evidence must not fail, got exit=%d", v.ExitCode)
	}
}

func TestHardConstraintIgnoresHardToolSequence(t *testing.T) {
	eng := NewEngine()
	res := &api.ReliabilityResult{
		ScenarioID: "sc_seq",
		Samples:    20,
		Passes:     0,
		Verdict:    api.VerdictFail,
		PerRunEvidence: []api.EvaluationResult{{
			EvaluatorName: "tool_sequence",
			Passed:        false,
			Criticality:   api.CriticalityHard,
		}},
	}
	pol := api.Policy{
		Name:            "prod",
		HardConstraints: api.HardConstraints{ForbiddenTools: 0, SchemaViolations: 0},
		Reliability:     api.PolicyReliability{DefaultMinimumPassRate: 0.95, OnFlaky: "warn"},
	}
	v := eng.Evaluate(pol, []*api.ReliabilityResult{res})
	if v.ExitCode != api.ExitFailure {
		t.Fatalf("reliability FAIL should still fail CI, got exit=%d", v.ExitCode)
	}
	if len(v.Violations) != 1 || v.Violations[0].ClauseType != "reliability" {
		t.Fatalf("hard tool_sequence must not trip hard_constraint clause, got %#v", v.Violations)
	}
}
