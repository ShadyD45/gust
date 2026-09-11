package stats

import (
	"fmt"
	"math"
)

// RecommendSamples returns the smallest n such that a perfect run (n/n) has
// Wilson lower bound >= minPassRate at the given confidence. Also returns the
// n needed so that an observed rate of expectedPassRate has lower bound
// >= minPassRate (when expectedPassRate >= minPassRate).
//
// Use this to avoid unattainable policies such as N=20 never PASS at P_min=0.95.
func RecommendSamples(minPassRate, confidence, expectedPassRate float64) (perfectN int, expectedN int, err error) {
	if minPassRate <= 0 || minPassRate >= 1 {
		return 0, 0, fmt.Errorf("minPassRate must be in (0, 1), got %v", minPassRate)
	}
	if expectedPassRate <= 0 || expectedPassRate > 1 {
		return 0, 0, fmt.Errorf("expectedPassRate must be in (0, 1], got %v", expectedPassRate)
	}
	if expectedPassRate < minPassRate {
		return 0, 0, fmt.Errorf("expectedPassRate %.4f is below minPassRate %.4f; PASS is unattainable at any n", expectedPassRate, minPassRate)
	}

	perfectN, err = searchMinN(func(n int) (bool, error) {
		ok, _, e := AttainablePASS(n, minPassRate, confidence)
		return ok, e
	})
	if err != nil {
		return 0, 0, err
	}

	expectedN, err = searchMinN(func(n int) (bool, error) {
		k := int(math.Floor(expectedPassRate*float64(n) + 1e-9))
		if k > n {
			k = n
		}
		iv, e := CalculateWilsonScore(k, n, confidence)
		if e != nil {
			return false, e
		}
		return iv.LowerBound >= minPassRate, nil
	})
	if err != nil {
		return perfectN, 0, err
	}
	return perfectN, expectedN, nil
}

func searchMinN(okAt func(n int) (bool, error)) (int, error) {
	const maxN = 100_000
	lo, hi := 1, 1
	for {
		ok, err := okAt(hi)
		if err != nil {
			return 0, err
		}
		if ok {
			break
		}
		if hi >= maxN {
			return 0, fmt.Errorf("no attainable n <= %d", maxN)
		}
		hi *= 2
		if hi > maxN {
			hi = maxN
		}
	}
	for lo < hi {
		mid := (lo + hi) / 2
		ok, err := okAt(mid)
		if err != nil {
			return 0, err
		}
		if ok {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo, nil
}
