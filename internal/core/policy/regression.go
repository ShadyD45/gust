package policy

import (
	"math"

	"gust/internal/core/stats"
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
	Regressed            bool     `json:"regressed"`
	BaselinePasses       int      `json:"baseline_passes,omitempty"`
	BaselineSamples      int      `json:"baseline_samples,omitempty"`
	BaselinePassRate     float64  `json:"baseline_pass_rate,omitempty"`
	CandidatePasses      int      `json:"candidate_passes,omitempty"`
	CandidateSamples     int      `json:"candidate_samples,omitempty"`
	CandidatePassRate    float64  `json:"candidate_pass_rate,omitempty"`
	PassRateDrop         float64  `json:"pass_rate_drop"`
	MaxPassRateDrop      float64  `json:"max_pass_rate_drop,omitempty"`
	LatencyIncreaseRatio *float64 `json:"latency_increase_ratio,omitempty"`
	LatencyComparable    bool     `json:"latency_comparable"`
	Significant          bool     `json:"significant"`
	PValue               float64  `json:"p_value"`
	CohensD              float64  `json:"cohens_d,omitempty"`
	EffectSize           string   `json:"effect_size,omitempty"`
	BootstrapDiffLower   float64  `json:"bootstrap_diff_lower,omitempty"`
	BootstrapDiffUpper   float64  `json:"bootstrap_diff_upper,omitempty"`
	BootstrapReplicates  int      `json:"bootstrap_replicates,omitempty"`
	Message              string   `json:"message"`
}

// CompareOptions tunes stats-v2 diagnostics on top of the z-test gate.
type CompareOptions struct {
	BootstrapReplicates int
	BootstrapSeed       int64
	Confidence          float64
}

func (o CompareOptions) effective() CompareOptions {
	if o.BootstrapReplicates <= 0 {
		o.BootstrapReplicates = 1000
	}
	if o.Confidence <= 0 || o.Confidence >= 1 {
		o.Confidence = 0.95
	}
	return o
}

// CompareRegression detects statistically meaningful regressions (z-test gate).
func CompareRegression(baseline, candidate ExperimentStats, policy api.PolicyRegression) RegressionResult {
	return CompareRegressionOpts(baseline, candidate, policy, CompareOptions{})
}

// CompareRegressionOpts is CompareRegression plus bootstrap CI and Cohen's d magnitude.
func CompareRegressionOpts(baseline, candidate ExperimentStats, policy api.PolicyRegression, opts CompareOptions) RegressionResult {
	opts = opts.effective()
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
	var latRatio *float64
	if latComparable {
		r := (candidate.LatencyNs - baseline.LatencyNs) / baseline.LatencyNs
		latRatio = &r
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

	d := stats.CohensDProportion(baseline.PassRate, candidate.PassRate)
	res := RegressionResult{
		BaselinePasses:       baseline.Passes,
		BaselineSamples:      baseline.Samples,
		BaselinePassRate:     baseline.PassRate,
		CandidatePasses:      candidate.Passes,
		CandidateSamples:     candidate.Samples,
		CandidatePassRate:    candidate.PassRate,
		PassRateDrop:         drop,
		MaxPassRateDrop:      maxDrop,
		LatencyIncreaseRatio: latRatio,
		LatencyComparable:    latComparable,
		Significant:          significant,
		PValue:               pValue,
		CohensD:              d,
		EffectSize:           stats.EffectSizeLabel(d),
		BootstrapReplicates:  opts.BootstrapReplicates,
	}

	if boot, err := stats.BootstrapRateDiffCI(
		baseline.Passes, baseline.Samples,
		candidate.Passes, candidate.Samples,
		opts.BootstrapReplicates, opts.Confidence, opts.BootstrapSeed,
	); err == nil {
		res.BootstrapDiffLower = boot.LowerBound
		res.BootstrapDiffUpper = boot.UpperBound
	}

	if drop > maxDrop && significant {
		res.Regressed = true
		res.Message = "statistically significant pass-rate regression"
		return res
	}
	if latComparable && latRatio != nil && *latRatio > maxLat && candidate.LatencyNs > baseline.LatencyNs {
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
