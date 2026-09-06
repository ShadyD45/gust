package stats

import (
	"testing"

	"gust/pkg/api"
)

func TestCalculateWilsonScore(t *testing.T) {
	// 1. Zero passes (k=0, n=20)
	intZero, err := CalculateWilsonScore(0, 20, 0.95)
	if err != nil {
		t.Fatalf("unexpected error on k=0: %v", err)
	}
	if intZero.LowerBound != 0.0 {
		t.Errorf("expected lower bound 0.0 for k=0, got %f", intZero.LowerBound)
	}
	if intZero.UpperBound <= 0.0 || intZero.UpperBound > 0.20 {
		t.Errorf("unexpected upper bound for k=0: %f", intZero.UpperBound)
	}

	// 2. All passed (k=20, n=20)
	intAll, err := CalculateWilsonScore(20, 20, 0.95)
	if err != nil {
		t.Fatalf("unexpected error on k=n: %v", err)
	}
	if intAll.UpperBound != 1.0 {
		t.Errorf("expected upper bound 1.0 for k=n, got %f", intAll.UpperBound)
	}
	if intAll.LowerBound < 0.80 {
		t.Errorf("unexpected lower bound for k=n: %f", intAll.LowerBound)
	}

	// 3. Middle rate (k=17, n=20 -> 85%)
	intMid, err := CalculateWilsonScore(17, 20, 0.95)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intMid.ObservedPassRate != 0.85 {
		t.Errorf("expected 0.85 pass rate, got %f", intMid.ObservedPassRate)
	}
	if intMid.LowerBound <= 0.50 || intMid.UpperBound >= 1.0 {
		t.Errorf("unexpected interval for 85%%: [%f, %f]", intMid.LowerBound, intMid.UpperBound)
	}
}

func TestClassifyVerdict(t *testing.T) {
	// Insufficient samples (< 5)
	int1, _ := CalculateWilsonScore(3, 3, 0.95)
	v1 := ClassifyVerdict(int1, 3, 0.95, 5)
	if v1 != api.VerdictInsufficientSamples {
		t.Errorf("expected INSUFFICIENT_SAMPLES for n=3, got %s", v1)
	}

	// Reliable pass: 100/100 -> PASS against 0.95 threshold
	// (N=20 can never PASS: even 20/20 has lower ≈ 0.84)
	intPass, _ := CalculateWilsonScore(100, 100, 0.95)
	vPass := ClassifyVerdict(intPass, 100, 0.95, 5)
	if vPass != api.VerdictPass {
		t.Errorf("expected PASS for 100/100, got %s (interval: [%f, %f])",
			vPass, intPass.LowerBound, intPass.UpperBound)
	}

	// Definite fail: 17/20 upper ≈ 0.948 < 0.95 -> FAIL
	intFail, _ := CalculateWilsonScore(17, 20, 0.95)
	vFail := ClassifyVerdict(intFail, 20, 0.95, 5)
	if vFail != api.VerdictFail {
		t.Errorf("expected FAIL for 17/20 against 0.95, got %s (interval: [%f, %f])",
			vFail, intFail.LowerBound, intFail.UpperBound)
	}

	// Flaky: 20/20 straddles 0.95 (lower ≈ 0.84, upper = 1.0)
	intFlaky, _ := CalculateWilsonScore(20, 20, 0.95)
	vFlaky := ClassifyVerdict(intFlaky, 20, 0.95, 5)
	if vFlaky != api.VerdictFlaky {
		t.Errorf("expected FLAKY for 20/20 against 0.95, got %s (interval: [%f, %f])",
			vFlaky, intFlaky.LowerBound, intFlaky.UpperBound)
	}

	// Also fail at larger N with low rate
	intFail100, _ := CalculateWilsonScore(40, 100, 0.95)
	vFail100 := ClassifyVerdict(intFail100, 100, 0.95, 5)
	if vFail100 != api.VerdictFail {
		t.Errorf("expected FAIL for 40/100, got %s", vFail100)
	}
}
