package stats

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

// BootstrapInterval is a percentile bootstrap confidence interval for a proportion.
type BootstrapInterval struct {
	LowerBound       float64 `json:"lower_bound"`
	UpperBound       float64 `json:"upper_bound"`
	ObservedPassRate float64 `json:"observed_pass_rate"`
	Replicates       int     `json:"replicates"`
	Seed             int64   `json:"seed"`
}

// BootstrapProportionCI resamples a Bernoulli series (k successes in n trials)
// with a seeded RNG. Wilson remains the default for Mode 3 verdicts; this is
// for compare / diagnostics.
func BootstrapProportionCI(passes, total, replicates int, confidence float64, seed int64) (BootstrapInterval, error) {
	if total <= 0 {
		return BootstrapInterval{}, ErrZeroSamples
	}
	if passes < 0 || passes > total {
		return BootstrapInterval{}, fmt.Errorf("passes must be between 0 and total")
	}
	if replicates < 100 {
		return BootstrapInterval{}, fmt.Errorf("replicates must be >= 100, got %d", replicates)
	}
	if confidence <= 0 || confidence >= 1 {
		return BootstrapInterval{}, fmt.Errorf("confidence must be in (0, 1), got %v", confidence)
	}

	rng := rand.New(rand.NewSource(seed))
	rates := make([]float64, replicates)
	for i := 0; i < replicates; i++ {
		successes := 0
		for j := 0; j < total; j++ {
			// Sample with replacement from the empirical Bernoulli(p=passes/total).
			if rng.Intn(total) < passes {
				successes++
			}
		}
		rates[i] = float64(successes) / float64(total)
	}
	sort.Float64s(rates)
	alpha := 1 - confidence
	loIdx := int(math.Floor(alpha / 2 * float64(replicates)))
	hiIdx := int(math.Ceil((1-alpha/2)*float64(replicates))) - 1
	if loIdx < 0 {
		loIdx = 0
	}
	if hiIdx >= replicates {
		hiIdx = replicates - 1
	}
	return BootstrapInterval{
		LowerBound:       rates[loIdx],
		UpperBound:       rates[hiIdx],
		ObservedPassRate: float64(passes) / float64(total),
		Replicates:       replicates,
		Seed:             seed,
	}, nil
}

// BootstrapRateDiffCI bootstraps the difference baseline_rate - candidate_rate.
func BootstrapRateDiffCI(basePasses, baseTotal, candPasses, candTotal, replicates int, confidence float64, seed int64) (BootstrapInterval, error) {
	if baseTotal <= 0 || candTotal <= 0 {
		return BootstrapInterval{}, ErrZeroSamples
	}
	if replicates < 100 {
		return BootstrapInterval{}, fmt.Errorf("replicates must be >= 100, got %d", replicates)
	}
	if confidence <= 0 || confidence >= 1 {
		return BootstrapInterval{}, fmt.Errorf("confidence must be in (0, 1), got %v", confidence)
	}

	rng := rand.New(rand.NewSource(seed))
	diffs := make([]float64, replicates)
	for i := 0; i < replicates; i++ {
		bOK := 0
		for j := 0; j < baseTotal; j++ {
			if rng.Intn(baseTotal) < basePasses {
				bOK++
			}
		}
		cOK := 0
		for j := 0; j < candTotal; j++ {
			if rng.Intn(candTotal) < candPasses {
				cOK++
			}
		}
		diffs[i] = float64(bOK)/float64(baseTotal) - float64(cOK)/float64(candTotal)
	}
	sort.Float64s(diffs)
	alpha := 1 - confidence
	loIdx := int(math.Floor(alpha / 2 * float64(replicates)))
	hiIdx := int(math.Ceil((1-alpha/2)*float64(replicates))) - 1
	if loIdx < 0 {
		loIdx = 0
	}
	if hiIdx >= replicates {
		hiIdx = replicates - 1
	}
	obs := float64(basePasses)/float64(baseTotal) - float64(candPasses)/float64(candTotal)
	return BootstrapInterval{
		LowerBound:       diffs[loIdx],
		UpperBound:       diffs[hiIdx],
		ObservedPassRate: obs,
		Replicates:       replicates,
		Seed:             seed,
	}, nil
}
