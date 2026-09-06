package mutate

import (
	"context"
	"fmt"

	"gust/internal/core/analyze"
	"gust/internal/ports"
	"gust/pkg/api"
)

// MutationBenchmarkReport summarizes Detection Rate and False Positive Rate.
type MutationBenchmarkReport struct {
	TotalMutantsGenerated int                    `json:"total_mutants_generated"`
	MutantsApplied        int                    `json:"mutants_applied"`
	MutantsSkipped        int                    `json:"mutants_skipped"`
	MutantsDetected       int                    `json:"mutants_detected"`
	DetectionRate         float64                `json:"detection_rate"`
	FalsePositiveRate     float64                `json:"false_positive_rate"`
	ClassBreakdown        map[string]*ClassStats `json:"class_breakdown"`
}

type ClassStats struct {
	Applied  int     `json:"applied"`
	Detected int     `json:"detected"`
	Rate     float64 `json:"rate"`
}

// Runner coordinates generating and evaluating mutants.
type Runner struct {
	mutators      []ports.Mutator
	analyzeEngine *analyze.Engine
}

// NewRunner creates a new Mutation Runner.
func NewRunner(mutators []ports.Mutator, evaluators []ports.Evaluator) *Runner {
	return &Runner{
		mutators:      mutators,
		analyzeEngine: analyze.NewEngine(evaluators),
	}
}

// GoldenCase combines a valid AgentRun with the assertions that test it.
type GoldenCase struct {
	Run        api.AgentRun
	Assertions []api.Assertion
}

// RunBenchmark executes mutation testing over golden cases.
func (r *Runner) RunBenchmark(ctx context.Context, goldenCases []GoldenCase) (*MutationBenchmarkReport, error) {
	report := &MutationBenchmarkReport{
		ClassBreakdown: make(map[string]*ClassStats),
	}

	// 1. Measure False Positive Rate on original unmutated golden cases
	unmutatedFailed := 0
	for _, gc := range goldenCases {
		res, err := r.analyzeEngine.AnalyzeRun(ctx, gc.Run, gc.Assertions, ports.EvaluationContext{})
		if err != nil {
			return nil, fmt.Errorf("evaluation failed on unmutated run %s: %w", gc.Run.RunID, err)
		}
		if !res.Passed {
			unmutatedFailed++
		}
	}

	if len(goldenCases) > 0 {
		report.FalsePositiveRate = float64(unmutatedFailed) / float64(len(goldenCases))
	}

	// 2. Measure Detection Rate on mutated runs
	for _, gc := range goldenCases {
		for _, m := range r.mutators {
			report.TotalMutantsGenerated++
			outcome, err := m.Mutate(ctx, gc.Run)
			if err != nil {
				return nil, fmt.Errorf("mutator %s failed: %w", m.Name(), err)
			}

			stats, ok := report.ClassBreakdown[m.Name()]
			if !ok {
				stats = &ClassStats{}
				report.ClassBreakdown[m.Name()] = stats
			}

			if outcome.Status == api.MutationSkipped {
				report.MutantsSkipped++
				continue
			}

			report.MutantsApplied++
			stats.Applied++

			// Evaluate mutated run against original assertions
			evalReport, err := r.analyzeEngine.AnalyzeRun(ctx, outcome.MutatedRun, gc.Assertions, ports.EvaluationContext{})
			if err != nil {
				return nil, fmt.Errorf("evaluating mutant from %s failed: %w", m.Name(), err)
			}

			// Mutant is killed/detected if evaluation fails (i.e. caught the injected defect)
			if !evalReport.Passed {
				report.MutantsDetected++
				stats.Detected++
			}
		}
	}

	if report.MutantsApplied > 0 {
		report.DetectionRate = float64(report.MutantsDetected) / float64(report.MutantsApplied)
	}

	for _, stats := range report.ClassBreakdown {
		if stats.Applied > 0 {
			stats.Rate = float64(stats.Detected) / float64(stats.Applied)
		}
	}

	return report, nil
}
