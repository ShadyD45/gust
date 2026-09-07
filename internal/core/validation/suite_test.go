package validation

import (
	"context"
	"testing"
)

func TestSuiteSize(t *testing.T) {
	n := len(AllCases())
	if n < 50 {
		t.Fatalf("suite too small: %d cases (want ≥ 50)", n)
	}
	if n > 120 {
		t.Fatalf("suite unexpectedly large: %d", n)
	}
	t.Logf("Gust Validation Suite size: %d", n)
}

func TestRunValidationSuite(t *testing.T) {
	rep, err := Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Logf("passed %d/%d", rep.Passed, rep.Total)
	for _, f := range rep.Failures {
		t.Logf("FAIL %s: %s (%s)", f.ID, f.Detail, f.Question)
	}
	if !rep.GatePassed {
		t.Fatalf("validation suite gate failed: %d failures", rep.Failed)
	}
}
