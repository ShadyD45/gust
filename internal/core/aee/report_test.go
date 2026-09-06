package aee

import (
	"context"
	"testing"
)

func TestRunSelfAEE(t *testing.T) {
	rep, err := Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Logf("AEE score=%.3f DR=%.2f FPR=%.2f thr=%.0f repro=%.2f h7=%v passed=%v",
		rep.Score, rep.DetectionRate, rep.FalsePositiveRate, rep.EvalThroughputCPS,
		rep.Reproducibility, rep.H7.AllCorrect, rep.Passed)
	if !rep.Passed {
		t.Fatalf("AEE gate failed: %v", rep.Notes)
	}
}
