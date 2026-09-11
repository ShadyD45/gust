package stats

import "math"

// CohensDProportion is the standardized mean difference for two Bernoulli
// proportions using the pooled standard deviation:
//
//	d = (p1 - p2) / sqrt(p_pool * (1 - p_pool))
//
// Returns 0 when the pooled variance is zero (identical all-pass or all-fail).
func CohensDProportion(p1, p2 float64) float64 {
	pool := (p1 + p2) / 2
	v := pool * (1 - pool)
	if v <= 0 {
		return 0
	}
	return (p1 - p2) / math.Sqrt(v)
}

// EffectSizeLabel maps |d| to conventional magnitude buckets.
func EffectSizeLabel(d float64) string {
	ad := math.Abs(d)
	switch {
	case ad < 0.2:
		return "negligible"
	case ad < 0.5:
		return "small"
	case ad < 0.8:
		return "medium"
	default:
		return "large"
	}
}
