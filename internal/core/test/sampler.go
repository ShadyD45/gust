package test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"gust/internal/core/analyze"
	"gust/internal/core/stats"
	"gust/internal/ports"
	"gust/pkg/api"
)

// ErrRunnerUnstable is returned when execution errors alone exceed the configured rate.
var ErrRunnerUnstable = errors.New("runner unstable: execution error rate exceeds max_execution_error_rate")

// FixtureProxyFactory starts an isolated mock-tool HTTP proxy for one sample's provider.
// Core stays adapter-free: CLI and the validation suite supply the fixtures adapter.
type FixtureProxyFactory func(provider ports.FixtureProvider) (endpoint string, closeFn func() error, err error)

// SamplingConfig configures a Mode 3 probabilistic test run.
type SamplingConfig struct {
	Scenario        api.TestScenario
	Runner          ports.TestRunner
	Evaluators      []ports.Evaluator
	Concurrency     int
	Endpoint        string                // runner-specific (ollama / http agent URL)
	FixtureEndpoint string                // shared mock proxy fallback when samples cannot be isolated
	FixtureProvider ports.FixtureProvider // optional; cloned per sample when possible
	MinSamples      int                   // override for INSUFFICIENT_SAMPLES; 0 → use policy default 5
	EvalContext     ports.EvaluationContext
	// MaxExecutionErrorRate aborts the scenario when exec errors / samples exceeds this.
	// Nil uses default 0.20; explicit 0 is zero tolerance.
	MaxExecutionErrorRate *float64
	// HasOrderedFixtures forces concurrency=1 when the provider cannot be cloned.
	HasOrderedFixtures bool
	// ProxyFactory, when set, starts a per-sample proxy bound to the cloned provider.
	ProxyFactory    FixtureProxyFactory
	Retry           api.RetryPolicy
	HardConstraints api.HardConstraints
}

// ClonableFixtureProvider is a FixtureProvider that can isolate per-sample state.
type ClonableFixtureProvider interface {
	ports.FixtureProvider
	Clone() ports.FixtureProvider
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

	_, canClone := cfg.FixtureProvider.(ClonableFixtureProvider)
	if cfg.HasOrderedFixtures && !canClone && concurrency > 1 {
		concurrency = 1
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
	maxExecRate := api.EffectiveMaxExecutionErrorRate(cfg.MaxExecutionErrorRate)
	retry := cfg.Retry.Effective()

	type slot struct {
		passed  bool
		evals   []api.EvaluationResult
		execErr error
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
				results[idx].execErr = ctx.Err()
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()

			fixtureEndpoint := cfg.FixtureEndpoint
			if fixtureEndpoint == "" {
				fixtureEndpoint = cfg.Endpoint
			}

			var sampleProvider ports.FixtureProvider
			if c, ok := cfg.FixtureProvider.(ClonableFixtureProvider); ok {
				sampleProvider = c.Clone()
			} else {
				sampleProvider = cfg.FixtureProvider
			}

			if cfg.ProxyFactory != nil && sampleProvider != nil && canClone {
				ep, closer, err := cfg.ProxyFactory(sampleProvider)
				if err != nil {
					results[idx].execErr = err
					return
				}
				if closer != nil {
					defer func() { _ = closer() }()
				}
				fixtureEndpoint = ep
			}

			reset := func() {
				if sampleProvider != nil {
					_ = sampleProvider.Reset()
				}
			}

			run, err := runWithRetry(ctx, cfg.Runner, cfg.Scenario, fixtureEndpoint, retry, reset)
			if err != nil {
				results[idx].execErr = err
				return
			}
			if sampleProvider != nil && concurrency == 1 && !canClone {
				_ = sampleProvider.Reset()
			}
			report, err := s.analyzeEngine.AnalyzeRun(ctx, run, cfg.Scenario.Assertions, withScenarioID(cfg.EvalContext, cfg.Scenario.ID))
			if err != nil {
				results[idx].execErr = err
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
					Criticality:      r.Criticality,
				}
			}
			results[idx].passed = report.Passed
			results[idx].evals = evals
		}(i)
	}
	wg.Wait()

	passes := 0
	execErrors := 0
	perRun := make([]api.EvaluationResult, 0, n)
	for _, sl := range results {
		if sl.execErr != nil {
			execErrors++
			continue
		}
		if sl.passed {
			passes++
		}
		perRun = append(perRun, sl.evals...)
	}

	forbidden, schema, other := api.CountPolicyHardFailures(perRun)
	hardFail := api.HardConstraintsExceeded(cfg.HardConstraints, forbidden, schema, other)

	if float64(execErrors)/float64(n) > maxExecRate {
		return nil, fmt.Errorf("%w: %d/%d samples (max rate %.2f)", ErrRunnerUnstable, execErrors, n, maxExecRate)
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
		ExecutionErrors:      execErrors,
	}, nil
}

func runWithRetry(ctx context.Context, runner ports.TestRunner, scenario api.TestScenario, fixtureEndpoint string, retry api.RetryPolicy, reset func()) (api.AgentRun, error) {
	var lastErr error
	for attempt := 1; attempt <= retry.MaxAttempts; attempt++ {
		if attempt > 1 && reset != nil {
			reset()
		}
		run, err := runner.Run(ctx, scenario, fixtureEndpoint)
		if err == nil {
			return run, nil
		}
		lastErr = err
		if attempt == retry.MaxAttempts || !ports.IsRetryable(err, retry.On) {
			return api.AgentRun{}, err
		}
		select {
		case <-ctx.Done():
			return api.AgentRun{}, ctx.Err()
		case <-time.After(time.Duration(retry.BackoffMs) * time.Millisecond):
		}
	}
	return api.AgentRun{}, lastErr
}

func withScenarioID(evalCtx ports.EvaluationContext, scenarioID string) ports.EvaluationContext {
	evalCtx.ScenarioID = scenarioID
	return evalCtx
}
