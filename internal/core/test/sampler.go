package test

import (
	"context"
	"fmt"
	"sync"

	"gust/internal/core/analyze"
	"gust/internal/core/stats"
	"gust/internal/ports"
	"gust/pkg/api"
)

// SamplingConfig configures a Mode 3 probabilistic test run.
type SamplingConfig struct {
	Scenario        api.TestScenario
	Runner          ports.TestRunner
	Evaluators      []ports.Evaluator
	Concurrency     int
	Endpoint        string                // runner-specific (ollama / http agent URL)
	FixtureEndpoint string                // mock tool proxy; passed to Runner.Run
	FixtureProvider ports.FixtureProvider // optional; Reset after each sample when concurrency is 1
	MinSamples      int                   // override for INSUFFICIENT_SAMPLES; 0 → use policy default 5
}

// Sampler orchestrates parallel sampling and Wilson classification.
type Sampler struct {
	analyzeEngine *analyze.Engine
}

// NewSampler creates a Mode 3 sampler.
func NewSampler(evaluators []ports.Evaluator) *Sampler {
	return &Sampler{analyzeEngine: analyze.NewEngine(evaluators)}
}

// RunScenario executes N independent runs and returns a ReliabilityResult.
func (s *Sampler) RunScenario(ctx context.Context, cfg SamplingConfig) (*api.ReliabilityResult, error) {
	if cfg.Runner == nil {
		return nil, fmt.Errorf("test runner is required")
	}
	n := cfg.Scenario.Reliability.Samples
	if n <= 0 {
		return nil, fmt.Errorf("reliability.samples must be > 0")
	}
	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 4
	}
	minSamples := cfg.MinSamples
	if minSamples <= 0 {
		minSamples = 5
	}
	minPass := cfg.Scenario.Reliability.MinimumPassRate
	confidence := cfg.Scenario.Reliability.Confidence
	if confidence <= 0 {
		confidence = 0.95
	}

	type slot struct {
		passed bool
		evals  []api.EvaluationResult
		err    error
	}
	results := make([]slot, n)
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				results[idx].err = ctx.Err()
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()

			fixtureEndpoint := cfg.FixtureEndpoint
			if fixtureEndpoint == "" {
				fixtureEndpoint = cfg.Endpoint
			}
			run, err := cfg.Runner.Run(ctx, cfg.Scenario, fixtureEndpoint)
			if err != nil {
				results[idx].err = err
				return
			}
			if cfg.FixtureProvider != nil && concurrency == 1 {
				_ = cfg.FixtureProvider.Reset()
			}
			report, err := s.analyzeEngine.AnalyzeRun(ctx, run, cfg.Scenario.Assertions, ports.EvaluationContext{
				ScenarioID: cfg.Scenario.ID,
			})
			if err != nil {
				results[idx].err = err
				return
			}
			evals := make([]api.EvaluationResult, len(report.Results))
			for j, r := range report.Results {
				evals[j] = api.EvaluationResult{
					EvaluatorName:    r.EvaluatorName,
					EvaluatorVersion: r.EvaluatorVersion,
					Passed:           r.Passed,
					Score:            r.Score,
					Message:          r.Message,
					Evidence:         r.Evidence,
					ExecutionTimeNs:  r.ExecutionTimeNs,
				}
			}
			results[idx].passed = report.Passed
			results[idx].evals = evals
		}(i)
	}
	wg.Wait()

	passes := 0
	perRun := make([]api.EvaluationResult, 0, n)
	hardFail := false
	for i, sl := range results {
		if sl.err != nil {
			return nil, fmt.Errorf("sample %d failed: %w", i, sl.err)
		}
		if sl.passed {
			passes++
		}
		for _, ev := range sl.evals {
			perRun = append(perRun, ev)
			if isHardConstraintFailure(ev) {
				hardFail = true
			}
		}
	}

	interval, err := stats.CalculateWilsonScore(passes, n, confidence)
	if err != nil {
		return nil, err
	}
	verdict := stats.ClassifyVerdict(interval, n, minPass, minSamples)

	return &api.ReliabilityResult{
		ScenarioID:           cfg.Scenario.ID,
		Samples:              n,
		Passes:               passes,
		ObservedPassRate:     interval.ObservedPassRate,
		ConfidenceInterval:   [2]float64{interval.LowerBound, interval.UpperBound},
		Verdict:              verdict,
		PerRunEvidence:       perRun,
		HardConstraintFailed: hardFail,
	}, nil
}

func isHardConstraintFailure(ev api.EvaluationResult) bool {
	if ev.Passed {
		return false
	}
	switch ev.EvaluatorName {
	case "forbidden_tool", "schema_validation":
		return true
	default:
		return false
	}
}
