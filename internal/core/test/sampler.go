package test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gust/internal/adapters/testrunner"
	"gust/internal/core/analyze"
	"gust/internal/core/stats"
	"gust/internal/ports"
	"gust/pkg/api"
)

// ErrRunnerUnstable is returned when infrastructure errors alone exceed the configured rate.
var ErrRunnerUnstable = errors.New("runner unstable: infrastructure error rate exceeds max_execution_error_rate")

// ErrUnattainablePASS is returned when N samples cannot mathematically reach PASS.
var ErrUnattainablePASS = errors.New("unattainable PASS: Wilson lower bound with perfect samples is below minimum_pass_rate")

// FixtureProxyFactory starts an isolated mock-tool HTTP proxy for one sample's provider.
type FixtureProxyFactory func(provider ports.FixtureProvider) (endpoint string, closeFn func() error, err error)

// SamplingConfig configures a Mode 3 probabilistic test run.
type SamplingConfig struct {
	Scenario              api.TestScenario
	Runner                ports.TestRunner
	Evaluators            []ports.Evaluator
	Concurrency           int
	Endpoint              string
	FixtureEndpoint       string
	FixtureProvider       ports.FixtureProvider
	MinSamples            int
	EvalContext           ports.EvaluationContext
	MaxExecutionErrorRate *float64
	HasOrderedFixtures    bool
	ProxyFactory          FixtureProxyFactory
	Retry                 api.RetryPolicy
	HardConstraints       api.HardConstraints
	EvaluationID          string
	OTelURL               string
	// FailUnattainable aborts before sampling when PASS is mathematically impossible.
	FailUnattainable bool
	// OnUnattainable is called with a warning when PASS is unattainable and FailUnattainable is false.
	OnUnattainable func(message string)
}

// ClonableFixtureProvider is a FixtureProvider that can isolate per-sample state.
type ClonableFixtureProvider interface {
	ports.FixtureProvider
	Clone() ports.FixtureProvider
}

// CallLedgerProvider optionally exposes per-sample fixture call evidence.
type CallLedgerProvider interface {
	CallLedger() []api.FixtureCallEvidence
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

	worldMode := cfg.Scenario.Environment.EffectiveWorldControl()
	if cfg.Scenario.Environment.WorldControl == "" && (cfg.FixtureProvider != nil || cfg.FixtureEndpoint != "") {
		// Sampling configs that supply fixtures/proxy endpoints are Gust-controlled
		// even when the scenario YAML omitted world_control.
		worldMode = api.WorldControlGust
	}
	isolate := fixtureIsolationReady(cfg) && worldMode == api.WorldControlGust
	if cfg.HasOrderedFixtures && !isolate && concurrency > 1 && worldMode == api.WorldControlGust {
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
	evaluationID := cfg.EvaluationID
	if evaluationID == "" {
		evaluationID = testrunner.NewEvaluationID()
	}

	if ok, lower, err := stats.AttainablePASS(n, minPass, confidence); err == nil && !ok {
		msg := fmt.Sprintf(
			"scenario %s: with samples=%d confidence=%.2f the best Wilson lower bound is %.4f < minimum_pass_rate=%.4f",
			cfg.Scenario.ID, n, confidence, lower, minPass,
		)
		if cfg.FailUnattainable {
			return nil, fmt.Errorf("%w: %s", ErrUnattainablePASS, msg)
		}
		if cfg.OnUnattainable != nil {
			cfg.OnUnattainable(msg)
		}
	}

	type slot struct {
		sample api.SampleResult
	}
	results := make([]slot, n)

	runOne := func(idx int) {
		fixtureEndpoint := cfg.FixtureEndpoint
		if fixtureEndpoint == "" {
			fixtureEndpoint = cfg.Endpoint
		}

		sampleProvider := cfg.FixtureProvider
		var closer func() error
		if isolate {
			c := cfg.FixtureProvider.(ClonableFixtureProvider)
			sampleProvider = c.Clone()
			ep, closeFn, err := cfg.ProxyFactory(sampleProvider)
			if err != nil {
				results[idx].sample = infraSample(evaluationID, cfg.Scenario.ID, "", api.FailureFixture, err)
				return
			}
			closer = closeFn
			fixtureEndpoint = ep
		}
		if closer != nil {
			defer func() { _ = closer() }()
		}

		req := testrunner.NewSampleRequest(evaluationID, cfg.Scenario, fixtureEndpoint, cfg.OTelURL)
		if worldMode == api.WorldControlExisting {
			req.FixtureEndpoint = ""
			req.WorldMode = api.WorldControlExisting
		} else {
			req.WorldMode = api.WorldControlGust
			req.FixtureEndpoint = fixtureEndpoint
		}

		reset := func() {
			if sampleProvider != nil {
				_ = sampleProvider.Reset()
			}
		}

		run, err := runWithRetry(ctx, cfg.Runner, req, retry, reset)
		if err != nil {
			results[idx].sample = classifyRunnerError(evaluationID, cfg.Scenario.ID, req, err)
			return
		}
		if !isolate && sampleProvider != nil {
			_ = sampleProvider.Reset()
		}

		report, err := s.analyzeEngine.AnalyzeRun(ctx, run, cfg.Scenario.Assertions, withScenarioID(cfg.EvalContext, cfg.Scenario.ID))
		if err != nil {
			results[idx].sample = api.SampleResult{
				SampleID:        req.SampleID,
				EvaluationID:    evaluationID,
				ScenarioID:      cfg.Scenario.ID,
				TraceID:         req.TraceID,
				RunID:           run.RunID,
				Status:          api.SampleStatusInfraError,
				FailureCategory: api.FailureEvaluator,
				Message:         err.Error(),
			}
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

		sample := api.SampleResult{
			SampleID:     req.SampleID,
			EvaluationID: evaluationID,
			ScenarioID:   cfg.Scenario.ID,
			TraceID:      req.TraceID,
			RunID:        run.RunID,
			Passed:       report.Passed,
			Evaluations:  evals,
		}
		if ledger, ok := sampleProvider.(CallLedgerProvider); ok {
			sample.FixtureCalls = ledger.CallLedger()
		}
		if run.Outcome.Status == "failed" || run.Outcome.Status == "timeout" || run.Outcome.Status == "cancelled" {
			sample.Passed = false
			sample.Status = api.SampleStatusAgentFailed
			sample.FailureCategory = api.FailureAgentRuntime
			sample.Message = run.Outcome.Error
			if sample.Message == "" {
				sample.Message = "agent outcome status=" + run.Outcome.Status
			}
		} else if report.Passed {
			sample.Status = api.SampleStatusPassed
		} else {
			sample.Status = api.SampleStatusFailed
			sample.FailureCategory = api.FailureAssertion
			sample.Message = "one or more hard assertions failed"
		}
		results[idx].sample = sample
	}

	if concurrency <= 1 {
		for i := 0; i < n; i++ {
			if err := ctx.Err(); err != nil {
				results[i].sample = infraSample(evaluationID, cfg.Scenario.ID, "", api.FailureTrigger, err)
				continue
			}
			runOne(i)
		}
	} else {
		sem := make(chan struct{}, concurrency)
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				select {
				case <-ctx.Done():
					results[idx].sample = infraSample(evaluationID, cfg.Scenario.ID, "", api.FailureTrigger, ctx.Err())
					return
				case sem <- struct{}{}:
				}
				defer func() { <-sem }()
				runOne(idx)
			}(i)
		}
		wg.Wait()
	}

	passes := 0
	completed := 0
	behavioralFailures := 0
	infraErrors := 0
	perRun := make([]api.EvaluationResult, 0, n)
	samples := make([]api.SampleResult, 0, n)

	for _, sl := range results {
		sample := sl.sample
		samples = append(samples, sample)
		if sample.FailureCategory.IsInfrastructure() {
			infraErrors++
			continue
		}
		completed++
		if sample.Passed {
			passes++
		} else {
			behavioralFailures++
		}
		perRun = append(perRun, sample.Evaluations...)
	}

	counts := api.CountPolicyHardFailures(perRun)
	hardFail := api.HardConstraintsExceeded(cfg.HardConstraints, counts)

	if float64(infraErrors)/float64(n) > maxExecRate {
		return nil, fmt.Errorf("%w: %d/%d samples (max rate %.2f)", ErrRunnerUnstable, infraErrors, n, maxExecRate)
	}

	var interval stats.WilsonInterval
	var verdict api.VerdictType
	if completed <= 0 {
		interval = stats.WilsonInterval{}
		verdict = api.VerdictInsufficientSamples
	} else {
		var err error
		interval, err = stats.CalculateWilsonScore(passes, completed, confidence)
		if err != nil {
			return nil, err
		}
		verdict = stats.ClassifyVerdict(interval, completed, minPass, minSamples)
	}

	return &api.ReliabilityResult{
		EvaluationID:         evaluationID,
		ScenarioID:           cfg.Scenario.ID,
		Samples:              n,
		SamplesRequested:     n,
		SamplesCompleted:     completed,
		Passes:               passes,
		BehavioralFailures:   behavioralFailures,
		InfrastructureErrors: infraErrors,
		ObservedPassRate:     interval.ObservedPassRate,
		ConfidenceInterval:   [2]float64{interval.LowerBound, interval.UpperBound},
		Verdict:              verdict,
		PerRunEvidence:       perRun,
		SampleResults:        samples,
		HardConstraintFailed: hardFail,
		ExecutionErrors:      infraErrors,
	}, nil
}

func runWithRetry(ctx context.Context, runner ports.TestRunner, req ports.SampleRequest, retry api.RetryPolicy, reset func()) (api.AgentRun, error) {
	var lastErr error
	for attempt := 1; attempt <= retry.MaxAttempts; attempt++ {
		if attempt > 1 && reset != nil {
			reset()
		}
		// Fresh sample identity on retries so concurrent correlation stays unambiguous.
		if attempt > 1 {
			req.SampleID = testrunner.NewSampleID(req.ScenarioID)
			req.TraceID = testrunner.NewTraceID()
			req.TraceParent = testrunner.BuildTraceParent(req.TraceID)
			req.Baggage = testrunner.BuildBaggage(req.EvaluationID, req.ScenarioID, req.SampleID)
		}
		run, err := runner.Run(ctx, req)
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

func classifyRunnerError(evaluationID, scenarioID string, req ports.SampleRequest, err error) api.SampleResult {
	cat := api.FailureTrigger
	msg := err.Error()
	lower := strings.ToLower(msg)
	switch {
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(lower, "timed out") || strings.Contains(lower, "timeout"):
		cat = api.FailureTimeout
	case strings.Contains(lower, "fixture"):
		cat = api.FailureFixture
	case strings.Contains(lower, "otel") || strings.Contains(lower, "trace") || strings.Contains(lower, "ingest") || strings.Contains(lower, "agent run"):
		cat = api.FailureTraceIngest
	}
	return api.SampleResult{
		SampleID:        req.SampleID,
		EvaluationID:    evaluationID,
		ScenarioID:      scenarioID,
		TraceID:         req.TraceID,
		Status:          api.SampleStatusInfraError,
		FailureCategory: cat,
		Message:         msg,
	}
}

func infraSample(evaluationID, scenarioID, sampleID string, cat api.FailureCategory, err error) api.SampleResult {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return api.SampleResult{
		SampleID:        sampleID,
		EvaluationID:    evaluationID,
		ScenarioID:      scenarioID,
		Status:          api.SampleStatusInfraError,
		FailureCategory: cat,
		Message:         msg,
	}
}

func withScenarioID(evalCtx ports.EvaluationContext, scenarioID string) ports.EvaluationContext {
	evalCtx.ScenarioID = scenarioID
	return evalCtx
}

func fixtureIsolationReady(cfg SamplingConfig) bool {
	if cfg.ProxyFactory == nil || cfg.FixtureProvider == nil {
		return false
	}
	_, ok := cfg.FixtureProvider.(ClonableFixtureProvider)
	return ok
}
