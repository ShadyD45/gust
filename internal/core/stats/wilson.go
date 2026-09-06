package stats

import (
	"errors"
	"math"

	"gust/pkg/api"
)

var (
	ErrZeroSamples = errors.New("sample count must be greater than zero")
)

// WilsonInterval represents the confidence interval bounds and observed rate.
type WilsonInterval struct {
	LowerBound       float64 `json:"lower_bound"`
	UpperBound       float64 `json:"upper_bound"`
	ObservedPassRate float64 `json:"observed_pass_rate"`
}

// CalculateWilsonScore computes the Wilson score interval for binomial proportion.
// It handles boundary edge cases (k=0, k=n) and strictly clamps bounds to [0.0, 1.0].
func CalculateWilsonScore(passes, total int, confidence float64) (WilsonInterval, error) {
	if total <= 0 {
		return WilsonInterval{}, ErrZeroSamples
	}
	if passes < 0 || passes > total {
		return WilsonInterval{}, errors.New("passes must be between 0 and total")
	}

	// Normal quantile z for given confidence level
	z := 1.95996 // Default 95%
	if confidence == 0.99 {
		z = 2.57583
	} else if confidence == 0.90 {
		z = 1.64485
	}

	n := float64(total)
	k := float64(passes)
	p := k / n
	z2 := z * z

	center := (k + z2/2.0) / (n + z2)
	margin := (z / (n + z2)) * math.Sqrt((k*(n-k)/n) + (z2/4.0))

	lower := math.Max(0.0, center-margin)
	upper := math.Min(1.0, center+margin)

	return WilsonInterval{
		LowerBound:       lower,
		UpperBound:       upper,
		ObservedPassRate: p,
	}, nil
}

// ClassifyVerdict maps the Wilson interval and sample count into a definitive verdict.
func ClassifyVerdict(interval WilsonInterval, total int, minPassRate float64, minSamples int) api.VerdictType {
	if total < minSamples {
		return api.VerdictInsufficientSamples
	}
	if interval.LowerBound >= minPassRate {
		return api.VerdictPass
	}
	if interval.UpperBound < minPassRate {
		return api.VerdictFail
	}
	return api.VerdictFlaky
}
