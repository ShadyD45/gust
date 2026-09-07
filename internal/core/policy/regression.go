package policy

import (
	"math"

	"gust/pkg/api"
)

// ExperimentStats summarizes an experiment for regression comparison.
type ExperimentStats struct {
	Name      string  `json:"name"`
	Passes    int     `json:"passes"`
	Samples   int     `json:"samples"`
	PassRate  float64 `json:"pass_rate"`
	LatencyNs float64 `json:"latency_ns"`
}

// RegressionResult describes baseline vs candidate comparison.
type RegressionResult struct {
	Regressed            bool    `json:"regressed"`
	PassRateDrop         float64 `json:"pass_rate_drop"`
	LatencyIncreaseRatio float64 `json:"latency_increase_ratio"`
	LatencyComparable    bool    `json:"latency_comparable"`
	Significant          bool    `json:"significant"`
	PValue               float64 `json:"p_value"`
	Message              string  `json:"message"`
}

// CompareRegression detects statistically meaningful regressions.
func CompareRegression(baseline, candidate ExperimentStats, policy api.PolicyRegression) RegressionResult {
	if baseline.Samples <= 0 || candidate.Samples <= 0 {
		return RegressionResult{Message: "insufficient samples for regression compare"}
	}
	if baseline.PassRate == 0 && baseline.Samples > 0 {
		baseline.PassRate = float64(baseline.Passes) / float64(baseline.Samples)
	}
	if candidate.PassRate == 0 && candidate.Samples > 0 {
		candidate.PassRate = float64(candidate.Passes) / float64(candidate.Samples)
	}

	drop := baseline.PassRate - candidate.PassRate
	latComparable := baseline.LatencyNs > 0
	latRatio := 0.0
	if latComparable {
		latRatio = (candidate.LatencyNs - baseline.LatencyNs) / baseline.LatencyNs
	}

	pValue := twoProportionPValue(baseline.Passes, baseline.Samples, candidate.Passes, candidate.Samples)
	significant := pValue < 0.05

	maxDrop := policy.MaxPassRateDrop
	if maxDrop <= 0 {
		maxDrop = 0.02
	}
	maxLat := policy.MaxLatencyIncreaseRatio
	if maxLat <= 0 {
		maxLat = 0.15
	}

	res := RegressionResult{
		PassRateDrop:         drop,
		LatencyIncreaseRatio: latRatio,
		LatencyComparable:    latComparable,
		Significant:          significant,
		PValue:               pValue,
	}

	if drop > maxDrop && significant {
		res.Regressed = true
		res.Message = "statistically significant pass-rate regression"
		return res
	}
	if latComparable && latRatio > maxLat && candidate.LatencyNs > baseline.LatencyNs {
		res.Regressed = true
		res.Message = "latency increase exceeds policy threshold"
		return res
	}
	if !latComparable {
		res.Message = "no significant pass-rate regression; latency regression not measurable (baseline latency unset)"
		return res
	}
	res.Message = "no significant regression"
	return res
}

// twoProportionPValue returns a two-sided p-value for H0: p1 == p2 (pooled z-test).
func twoProportionPValue(k1, n1, k2, n2 int) float64 {
	if n1 <= 0 || n2 <= 0 {
		return 1.0
	}
	p1 := float64(k1) / float64(n1)
	p2 := float64(k2) / float64(n2)
	p := float64(k1+k2) / float64(n1+n2)
	se := math.Sqrt(p * (1 - p) * (1/float64(n1) + 1/float64(n2)))
	if se == 0 {
		if p1 == p2 {
			return 1.0
		}
		return 0.0
	}
	z := math.Abs(p1-p2) / se
	// Two-sided: 2 * (1 - Φ(z))
	return 2 * (1 - normalCDF(z))
}

func normalCDF(z float64) float64 {
	return 0.5 * (1 + math.Erf(z/math.Sqrt2))
}

// EvaluateWithRegression merges reliability policy with an optional regression result.
func (e *Engine) EvaluateWithRegression(policy api.Policy, results []*api.ReliabilityResult, regression *RegressionResult) Verdict {
	v := e.Evaluate(policy, results)
	if regression != nil && regression.Regressed && v.ExitCode != api.ExitFailure {
		v.OverallVerdict = api.VerdictFail
		v.ExitCode = api.ExitFailure
		v.Violations = append(v.Violations, PolicyViolation{
			ClauseType: "regression",
			Severity:   "fatal",
			Message:    regression.Message,
		})
	}
	return v
}
