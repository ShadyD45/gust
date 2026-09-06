package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"gust/internal/core/stats"
	"gust/internal/ports"
	"gust/pkg/api"
	"gust/pkg/jcs"
)

// CalibrationCase is one human-labeled open-ended judgement pair.
type CalibrationCase struct {
	ID         string       `json:"id"`
	Rubric     string       `json:"rubric"`
	HumanScore float64      `json:"human_score"`
	Threshold  float64      `json:"threshold,omitempty"`
	Run        api.AgentRun `json:"run"`
}

// CalibrationDataset is a versioned, content-addressable set of labeled cases.
type CalibrationDataset struct {
	Version     string            `json:"version"`
	Description string            `json:"description,omitempty"`
	Cases       []CalibrationCase `json:"cases"`
	ContentHash string            `json:"content_hash,omitempty"`
}

// CalibrationReport summarizes Spearman correlation of a judge vs human labels.
type CalibrationReport struct {
	DatasetHash   string    `json:"dataset_hash"`
	Provider      string    `json:"provider"`
	Model         string    `json:"model,omitempty"`
	N             int       `json:"n"`
	SpearmanRho   float64   `json:"spearman_rho"`
	MinSpearman   float64   `json:"min_spearman"`
	Passed        bool      `json:"passed"`
	Experimental  bool      `json:"experimental"`
	GeneratedAt   time.Time `json:"generated_at"`
	HumanScores   []float64 `json:"human_scores,omitempty"`
	JudgeScores   []float64 `json:"judge_scores,omitempty"`
	CaseIDs       []string  `json:"case_ids,omitempty"`
	Errors        []string  `json:"errors,omitempty"`
}

const DefaultMinSpearman = 0.7
const DefaultMinCalibrationCases = 50

// LoadDataset reads a calibration JSON file and verifies/fills its content hash.
func LoadDataset(path string) (*CalibrationDataset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ds CalibrationDataset
	if err := json.Unmarshal(data, &ds); err != nil {
		return nil, fmt.Errorf("parse calibration dataset: %w", err)
	}
	if len(ds.Cases) == 0 {
		return nil, fmt.Errorf("calibration dataset has no cases")
	}
	hash, err := jcs.ContentHash(map[string]any{
		"version": ds.Version,
		"cases":   ds.Cases,
	})
	if err != nil {
		return nil, err
	}
	ds.ContentHash = hash
	return &ds, nil
}

// Calibrate runs the provider over every case and computes Spearman ρ.
func Calibrate(ctx context.Context, ds *CalibrationDataset, provider ports.JudgeProvider, minRho float64) (*CalibrationReport, error) {
	if minRho <= 0 {
		minRho = DefaultMinSpearman
	}
	human := make([]float64, 0, len(ds.Cases))
	model := make([]float64, 0, len(ds.Cases))
	ids := make([]string, 0, len(ds.Cases))
	var errs []string

	for _, c := range ds.Cases {
		threshold := c.Threshold
		if threshold <= 0 {
			threshold = 0.7
		}
		jr, err := provider.Judge(ctx, ports.JudgeRequest{
			Run:       c.Run,
			Rubric:    c.Rubric,
			Threshold: threshold,
		})
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", c.ID, err))
			continue
		}
		human = append(human, c.HumanScore)
		model = append(model, jr.Score)
		ids = append(ids, c.ID)
	}

	report := &CalibrationReport{
		DatasetHash:  ds.ContentHash,
		Provider:     provider.Name(),
		N:            len(human),
		MinSpearman:  minRho,
		GeneratedAt:  time.Now().UTC(),
		HumanScores:  human,
		JudgeScores:  model,
		CaseIDs:      ids,
		Errors:       errs,
		Experimental: true,
	}
	if len(human) < 2 {
		return report, fmt.Errorf("calibration needs ≥2 successful judgements, got %d (%d errors)", len(human), len(errs))
	}
	rho, err := stats.SpearmanRho(human, model)
	if err != nil {
		return report, err
	}
	report.SpearmanRho = rho
	report.Passed = rho >= minRho && len(ds.Cases) >= DefaultMinCalibrationCases && len(errs) == 0
	report.Experimental = !report.Passed
	return report, nil
}
