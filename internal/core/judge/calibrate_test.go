package judge_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	adapterjudge "gust/internal/adapters/judge"
	corejudge "gust/internal/core/judge"
	"gust/internal/ports"
)

func TestSeedDatasetSize(t *testing.T) {
	ds := corejudge.SeedDataset()
	if len(ds.Cases) < corejudge.DefaultMinCalibrationCases {
		t.Fatalf("seed dataset has %d cases, want ≥%d", len(ds.Cases), corejudge.DefaultMinCalibrationCases)
	}
}

func TestCalibrateMockAligned(t *testing.T) {
	ds := corejudge.SeedDataset()
	// ScoreFn mirrors human bands so Spearman ρ is high (proves the harness).
	provider := &adapterjudge.MockProvider{
		ScoreFn: func(req ports.JudgeRequest) ports.JudgeResult {
			score := 0.5
			out := req.Run.Outcome.Output
			status := req.Run.Outcome.Status
			switch {
			case status != "completed":
				score = 0.1
			case len(out) > 60:
				score = 0.95
			case len(out) > 40:
				score = 0.8
			case len(out) > 15:
				score = 0.55
			default:
				score = 0.35
			}
			return ports.JudgeResult{Score: score, Passed: score >= 0.7, Rationale: "aligned mock", Model: "mock"}
		},
	}
	report, err := corejudge.Calibrate(context.Background(), ds, provider, corejudge.DefaultMinSpearman)
	if err != nil {
		t.Fatalf("Calibrate: %v", err)
	}
	if report.SpearmanRho < 0.7 {
		t.Fatalf("expected ρ ≥ 0.7 with aligned mock, got %v", report.SpearmanRho)
	}
	if !report.Passed {
		t.Fatalf("expected calibration pass, report=%+v", report)
	}
}

func TestWriteSeedDatasetArtifact(t *testing.T) {
	// Ensures testdata stays in sync when regenerate is requested.
	root := filepath.Join("..", "..", "..", "testdata", "judge", "calibration")
	path := filepath.Join(root, "v1.json")
	if _, err := os.Stat(path); err == nil {
		ds, err := corejudge.LoadDataset(path)
		if err != nil {
			t.Fatalf("LoadDataset: %v", err)
		}
		if len(ds.Cases) < corejudge.DefaultMinCalibrationCases {
			t.Fatalf("checked-in dataset too small: %d", len(ds.Cases))
		}
		return
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	ds := corejudge.SeedDataset()
	data, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
