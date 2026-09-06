package stats

import (
	"fmt"
	"math"
	"sort"
)

// SpearmanRho computes the Spearman rank correlation coefficient between two
// equal-length series. Ties use average ranks. Returns an error if n < 2.
func SpearmanRho(x, y []float64) (float64, error) {
	if len(x) != len(y) {
		return 0, fmt.Errorf("spearman: length mismatch %d vs %d", len(x), len(y))
	}
	n := len(x)
	if n < 2 {
		return 0, fmt.Errorf("spearman: need at least 2 pairs, got %d", n)
	}
	rx := ranks(x)
	ry := ranks(y)
	return pearson(rx, ry), nil
}

func ranks(vals []float64) []float64 {
	type indexed struct {
		i int
		v float64
	}
	items := make([]indexed, len(vals))
	for i, v := range vals {
		items[i] = indexed{i: i, v: v}
	}
	sort.Slice(items, func(a, b int) bool {
		if items[a].v == items[b].v {
			return items[a].i < items[b].i
		}
		return items[a].v < items[b].v
	})
	out := make([]float64, len(vals))
	for i := 0; i < len(items); {
		j := i + 1
		for j < len(items) && items[j].v == items[i].v {
			j++
		}
		// average rank (1-based)
		avg := 0.0
		for k := i; k < j; k++ {
			avg += float64(k + 1)
		}
		avg /= float64(j - i)
		for k := i; k < j; k++ {
			out[items[k].i] = avg
		}
		i = j
	}
	return out
}

func pearson(x, y []float64) float64 {
	n := float64(len(x))
	var sumX, sumY, sumXX, sumYY, sumXY float64
	for i := range x {
		sumX += x[i]
		sumY += y[i]
		sumXX += x[i] * x[i]
		sumYY += y[i] * y[i]
		sumXY += x[i] * y[i]
	}
	num := n*sumXY - sumX*sumY
	den := math.Sqrt((n*sumXX - sumX*sumX) * (n*sumYY - sumY*sumY))
	if den == 0 {
		return 0
	}
	return num / den
}
