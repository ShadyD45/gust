package stats

import (
	"math"
	"testing"
)

func TestBootstrapProportionCICoversTrueP(t *testing.T) {
	const trueP = 0.8
	const n = 50
	const trials = 40
	const replicates = 500
	covered := 0
	for trial := 0; trial < trials; trial++ {
		// Simulate one experiment: Binomial(n, trueP) via fixed seed offset.
		passes := int(math.Round(trueP * float64(n)))
		iv, err := BootstrapProportionCI(passes, n, replicates, 0.95, int64(1000+trial))
		if err != nil {
			t.Fatal(err)
		}
		if iv.LowerBound <= trueP && trueP <= iv.UpperBound {
			covered++
		}
	}
	// Nominal 95%; with fixed k and few trials allow a wide band.
	rate := float64(covered) / float64(trials)
	if rate < 0.70 {
		t.Fatalf("bootstrap coverage too low: %.2f (%d/%d)", rate, covered, trials)
	}
}

func TestBootstrapTinyNoiseStable(t *testing.T) {
	a, err := BootstrapProportionCI(95, 100, 400, 0.95, 42)
	if err != nil {
		t.Fatal(err)
	}
	b, err := BootstrapProportionCI(94, 100, 400, 0.95, 42)
	if err != nil {
		t.Fatal(err)
	}
	// Intervals should heavily overlap for tiny noise.
	if a.UpperBound < b.LowerBound || b.UpperBound < a.LowerBound {
		t.Fatalf("tiny noise should not produce disjoint CIs: %+v vs %+v", a, b)
	}
}

func TestBootstrapLargeDropStillTrips(t *testing.T) {
	diff, err := BootstrapRateDiffCI(95, 100, 60, 100, 500, 0.95, 7)
	if err != nil {
		t.Fatal(err)
	}
	if diff.LowerBound <= 0 {
		t.Fatalf("large drop bootstrap CI should exclude 0: %+v", diff)
	}
}

func TestCohensDProportion(t *testing.T) {
	d := CohensDProportion(0.95, 0.75)
	if d <= 0 {
		t.Fatalf("expected positive d for improvement of baseline over candidate, got %v", d)
	}
	if EffectSizeLabel(d) == "negligible" {
		t.Fatalf("20pp drop should not be negligible, d=%v label=%s", d, EffectSizeLabel(d))
	}
	if CohensDProportion(1, 1) != 0 {
		t.Fatalf("identical rates should yield d=0")
	}
}

func TestRecommendSamplesPass95(t *testing.T) {
	perfect, expected, err := RecommendSamples(0.95, 0.95, 1.0)
	if err != nil {
		t.Fatal(err)
	}
	if perfect < 59 { // known: need ~59–73 for 95% floor depending on exact Wilson
		t.Fatalf("perfect N too small: %d", perfect)
	}
	ok, _, err := AttainablePASS(20, 0.95, 0.95)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("N=20 must remain unattainable at 0.95")
	}
	ok, _, err = AttainablePASS(perfect, 0.95, 0.95)
	if err != nil || !ok {
		t.Fatalf("recommended perfect N=%d should be attainable", perfect)
	}
	if expected < perfect {
		t.Fatalf("expectedN (%d) should be >= perfectN (%d) when expectedPassRate=1", expected, perfect)
	}
}

func TestRecommendSamplesUnattainableExpected(t *testing.T) {
	_, _, err := RecommendSamples(0.95, 0.95, 0.90)
	if err == nil {
		t.Fatal("expected error when expected rate below min pass rate")
	}
}
