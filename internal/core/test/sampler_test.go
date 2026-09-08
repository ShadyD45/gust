package test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"gust/internal/adapters/evaluators"
	"gust/internal/adapters/fixtures"
	"gust/internal/adapters/testrunner"
	"gust/internal/ports"
	"gust/pkg/api"
)

func baseScenario(id string, samples int) api.TestScenario {
	return api.TestScenario{
		ID:          id,
		Version:     "1.0",
		Description: "H7 synthetic reliability",
		Task:        api.TaskInfo{ID: "t1", Input: "complete the task"},
		Assertions: []api.Assertion{{
			ID:   "assert_success",
			Type: api.AssertTaskSuccess,
		}},
		Reliability: api.ReliabilityConfig{
			Samples:         samples,
			MinimumPassRate: 0.95,
			Confidence:      0.95,
		},
		Provenance: api.TestScenarioProvenance{
			Source:      "unit_test",
			ExtractedAt: time.Now().UTC(),
		},
	}
}

func builtinEvals() []ports.Evaluator {
	return []ports.Evaluator{&evaluators.TaskSuccessEvaluator{}}
}

func TestH7_PassAtN100(t *testing.T) {
	runner := testrunner.NewSyntheticRunner(1.0, 42)
	sampler := NewSampler(builtinEvals())
	res, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario:    baseScenario("h7_pass", 100),
		Runner:      runner,
		Evaluators:  builtinEvals(),
		Concurrency: 8,
	})
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if res.Verdict != api.VerdictPass {
		t.Fatalf("expected PASS for 100/100, got %s (rate=%.3f ci=[%.3f,%.3f])",
			res.Verdict, res.ObservedPassRate, res.ConfidenceInterval[0], res.ConfidenceInterval[1])
	}
}

func TestH7_FlakyAtN20Perfect(t *testing.T) {
	runner := testrunner.NewSyntheticRunner(1.0, 7)
	sampler := NewSampler(builtinEvals())
	res, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario:    baseScenario("h7_flaky", 20),
		Runner:      runner,
		Concurrency: 4,
	})
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if res.Verdict != api.VerdictFlaky {
		t.Fatalf("expected FLAKY for 20/20 vs 0.95, got %s (ci=[%.3f,%.3f])",
			res.Verdict, res.ConfidenceInterval[0], res.ConfidenceInterval[1])
	}
}

func TestH7_FailAtLowRate(t *testing.T) {
	runner := testrunner.NewSyntheticRunner(0.0, 99)
	sampler := NewSampler(builtinEvals())
	res, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario:    baseScenario("h7_fail", 20),
		Runner:      runner,
		Concurrency: 4,
	})
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if res.Verdict != api.VerdictFail {
		t.Fatalf("expected FAIL for 0/20, got %s", res.Verdict)
	}
}

func TestH7_FailSeventeenOfTwenty(t *testing.T) {
	outcomes := make([]bool, 20)
	for i := 0; i < 17; i++ {
		outcomes[i] = true
	}
	runner := testrunner.NewSyntheticRunner(0, 1).WithFixedOutcomes(outcomes)
	sampler := NewSampler(builtinEvals())
	res, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario: baseScenario("h7_fail_17", 20),
		Runner:   runner,
	})
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if res.Passes != 17 {
		t.Fatalf("expected 17 passes, got %d", res.Passes)
	}
	if res.Verdict != api.VerdictFail {
		t.Fatalf("expected FAIL for 17/20, got %s (ci=[%.3f,%.3f])",
			res.Verdict, res.ConfidenceInterval[0], res.ConfidenceInterval[1])
	}
}

// errOnceRunner fails the first call for selected sample indices, then succeeds.
type errOnceRunner struct {
	inner      ports.TestRunner
	failFirstN int
	calls      atomic.Int32
}

func (r *errOnceRunner) Name() string { return "err_once" }

func (r *errOnceRunner) Run(ctx context.Context, req ports.SampleRequest) (api.AgentRun, error) {
	n := int(r.calls.Add(1))
	if n <= r.failFirstN {
		return api.AgentRun{}, fmt.Errorf("infra sample %d", n)
	}
	return r.inner.Run(ctx, req)
}

func TestExecErrorCountsAsFailedSample(t *testing.T) {
	inner := testrunner.NewSyntheticRunner(1.0, 1)
	runner := &errOnceRunner{inner: inner, failFirstN: 1}
	sampler := NewSampler(builtinEvals())
	res, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario:              baseScenario("exec_err", 5),
		Runner:                runner,
		Concurrency:           1,
		MaxExecutionErrorRate: api.Float64Ptr(0.5),
		MinSamples:            1,
		Retry:                 api.RetryPolicy{MaxAttempts: 1, On: api.RetryOnNone},
	})
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if res.ExecutionErrors != 1 {
		t.Fatalf("expected 1 execution error, got %d", res.ExecutionErrors)
	}
	if res.Passes != 4 {
		t.Fatalf("expected 4 passes, got %d", res.Passes)
	}
	if res.Samples != 5 {
		t.Fatalf("expected samples=5, got %d", res.Samples)
	}
}

func TestExecErrorRateUnstable(t *testing.T) {
	inner := testrunner.NewSyntheticRunner(1.0, 1)
	runner := &errOnceRunner{inner: inner, failFirstN: 3}
	sampler := NewSampler(builtinEvals())
	_, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario:              baseScenario("unstable", 5),
		Runner:                runner,
		Concurrency:           1,
		MaxExecutionErrorRate: api.Float64Ptr(0.20),
		Retry:                 api.RetryPolicy{MaxAttempts: 1, On: api.RetryOnNone},
	})
	if err == nil || !errors.Is(err, ErrRunnerUnstable) {
		t.Fatalf("expected ErrRunnerUnstable, got %v", err)
	}
}

func TestMaxExecutionErrorRateZeroAborts(t *testing.T) {
	inner := testrunner.NewSyntheticRunner(1.0, 1)
	runner := &errOnceRunner{inner: inner, failFirstN: 1}
	sampler := NewSampler(builtinEvals())
	_, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario:              baseScenario("zero_tol", 5),
		Runner:                runner,
		Concurrency:           1,
		MaxExecutionErrorRate: api.Float64Ptr(0),
		Retry:                 api.RetryPolicy{MaxAttempts: 1, On: api.RetryOnNone},
	})
	if err == nil || !errors.Is(err, ErrRunnerUnstable) {
		t.Fatalf("expected ErrRunnerUnstable at rate 0, got %v", err)
	}
}

type transientOnceRunner struct {
	inner ports.TestRunner
	calls atomic.Int32
}

func (r *transientOnceRunner) Name() string { return "transient_once" }

func (r *transientOnceRunner) Run(ctx context.Context, req ports.SampleRequest) (api.AgentRun, error) {
	if r.calls.Add(1) == 1 {
		return api.AgentRun{}, fmt.Errorf("%w: blip", ports.ErrTransient)
	}
	return r.inner.Run(ctx, req)
}

func TestRetryTransientThenSuccess(t *testing.T) {
	inner := testrunner.NewSyntheticRunner(1.0, 1)
	runner := &transientOnceRunner{inner: inner}
	sampler := NewSampler(builtinEvals())
	res, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario:    baseScenario("retry_ok", 1),
		Runner:      runner,
		Concurrency: 1,
		MinSamples:  1,
		Retry:       api.RetryPolicy{MaxAttempts: 2, On: api.RetryOnTransient, BackoffMs: 1},
	})
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if res.ExecutionErrors != 0 || res.Passes != 1 {
		t.Fatalf("expected retry to succeed, errors=%d passes=%d", res.ExecutionErrors, res.Passes)
	}
}

func TestRetryNoneDoesNotRetryTransient(t *testing.T) {
	inner := testrunner.NewSyntheticRunner(1.0, 1)
	runner := &transientOnceRunner{inner: inner}
	sampler := NewSampler(builtinEvals())
	res, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario:              baseScenario("retry_none", 1),
		Runner:                runner,
		Concurrency:           1,
		MinSamples:            1,
		MaxExecutionErrorRate: api.Float64Ptr(1),
		Retry:                 api.RetryPolicy{MaxAttempts: 2, On: api.RetryOnNone},
	})
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if res.ExecutionErrors != 1 {
		t.Fatalf("expected 1 exec error without retry, got %d", res.ExecutionErrors)
	}
}

func testProxyFactory(p ports.FixtureProvider) (string, func() error, error) {
	proxy, err := fixtures.NewMockToolProxyServer(p)
	if err != nil {
		return "", nil, err
	}
	return proxy.Start(), proxy.Close, nil
}

func orderedPollFixtures() []api.Fixture {
	return []api.Fixture{
		{
			FixtureID:        "fx_poll_1",
			Tool:             "poll",
			MatchStrategy:    api.MatchStrategyOrderedSequence,
			RecordedResponse: api.RecordedResponse{Status: "success", Body: "PENDING"},
			Provenance:       api.ProvenanceRecorded,
		},
		{
			FixtureID:        "fx_poll_2",
			Tool:             "poll",
			MatchStrategy:    api.MatchStrategyOrderedSequence,
			RecordedResponse: api.RecordedResponse{Status: "success", Body: "DONE"},
			Provenance:       api.ProvenanceRecorded,
		},
	}
}

func TestOrderedFixturesConcurrentIsolation(t *testing.T) {
	provider := fixtures.NewMemoryFixtureProvider()
	if err := provider.LoadFixtures(orderedPollFixtures()); err != nil {
		t.Fatal(err)
	}
	runner := &testrunner.FixtureProbeRunner{
		Calls: []ports.ToolCall{{Name: "poll"}, {Name: "poll"}},
	}
	sc := baseScenario("ordered_iso", 20)
	sc.Assertions = []api.Assertion{{ID: "ok", Type: api.AssertTaskSuccess}}
	sampler := NewSampler(builtinEvals())
	res, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario:           sc,
		Runner:             runner,
		Concurrency:        4,
		FixtureProvider:    provider,
		HasOrderedFixtures: true,
		ProxyFactory:       testProxyFactory,
		MinSamples:         1,
	})
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if res.ExecutionErrors != 0 {
		t.Fatalf("exec errors: %d", res.ExecutionErrors)
	}
	if res.Passes != 20 {
		t.Fatalf("expected 20 isolated passes, got %d", res.Passes)
	}
}

type sharedSeqProvider struct {
	seq []string
	idx int
}

func (p *sharedSeqProvider) Lookup(_ context.Context, call ports.ToolCall) (api.RecordedResponse, bool, error) {
	if call.Name != "poll" {
		return api.RecordedResponse{}, false, nil
	}
	if p.idx >= len(p.seq) {
		return api.RecordedResponse{}, false, nil
	}
	body := p.seq[p.idx]
	p.idx++
	return api.RecordedResponse{Status: "success", Body: body}, true, nil
}

func (p *sharedSeqProvider) Record(context.Context, ports.ToolCall, api.RecordedResponse) error {
	return nil
}

func (p *sharedSeqProvider) Reset() error {
	p.idx = 0
	return nil
}

func TestOrderedFixturesForceSerialWithoutClone(t *testing.T) {
	provider := &sharedSeqProvider{seq: []string{"PENDING", "DONE"}}
	proxy, err := fixtures.NewMockToolProxyServer(provider)
	if err != nil {
		t.Fatal(err)
	}
	endpoint := proxy.Start()
	t.Cleanup(func() { _ = proxy.Close() })

	runner := &testrunner.FixtureProbeRunner{
		Calls: []ports.ToolCall{{Name: "poll"}, {Name: "poll"}},
	}
	sc := baseScenario("ordered_noclone", 8)
	sampler := NewSampler(builtinEvals())
	res, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario:           sc,
		Runner:             runner,
		Concurrency:        4, // must be forced serial
		FixtureProvider:    provider,
		FixtureEndpoint:    endpoint,
		HasOrderedFixtures: true,
		MinSamples:         1,
	})
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if res.Passes != 8 || res.ExecutionErrors != 0 {
		t.Fatalf("serial fallback should isolate via Reset: passes=%d errors=%d", res.Passes, res.ExecutionErrors)
	}
}

func TestOrderedFixturesForceSerialWithoutProxyFactory(t *testing.T) {
	provider := fixtures.NewMemoryFixtureProvider()
	if err := provider.LoadFixtures(orderedPollFixtures()); err != nil {
		t.Fatal(err)
	}
	proxy, err := fixtures.NewMockToolProxyServer(provider)
	if err != nil {
		t.Fatal(err)
	}
	endpoint := proxy.Start()
	t.Cleanup(func() { _ = proxy.Close() })

	runner := &testrunner.FixtureProbeRunner{
		Calls: []ports.ToolCall{{Name: "poll"}, {Name: "poll"}},
	}
	sc := baseScenario("ordered_noproxy", 12)
	sampler := NewSampler(builtinEvals())
	res, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario:           sc,
		Runner:             runner,
		Concurrency:        4,
		FixtureProvider:    provider, // clonable, but no ProxyFactory
		FixtureEndpoint:    endpoint,
		HasOrderedFixtures: true,
		MinSamples:         1,
	})
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if res.Passes != 12 || res.ExecutionErrors != 0 {
		t.Fatalf("missing ProxyFactory must not race: passes=%d errors=%d", res.Passes, res.ExecutionErrors)
	}
}

func TestNonTransientAgentFailureNotRetried(t *testing.T) {
	inner := testrunner.NewSyntheticRunner(1.0, 1)
	runner := &errOnceRunner{inner: inner, failFirstN: 1}
	sampler := NewSampler(builtinEvals())
	res, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario:              baseScenario("agent_crash", 1),
		Runner:                runner,
		Concurrency:           1,
		MinSamples:            1,
		MaxExecutionErrorRate: api.Float64Ptr(1),
		Retry:                 api.RetryPolicy{MaxAttempts: 3, On: api.RetryOnTransient, BackoffMs: 1},
	})
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if res.ExecutionErrors != 1 || res.Passes != 0 {
		t.Fatalf("generic agent error must not retry: errors=%d passes=%d", res.ExecutionErrors, res.Passes)
	}
}

type capturingRunner struct {
	inner ports.TestRunner
	runs  []api.AgentRun
}

func (r *capturingRunner) Name() string { return "capturing" }

func (r *capturingRunner) Run(ctx context.Context, req ports.SampleRequest) (api.AgentRun, error) {
	run, err := r.inner.Run(ctx, req)
	if err != nil {
		return api.AgentRun{}, err
	}
	r.runs = append(r.runs, run)
	return run, nil
}

func TestConcurrencyOneMapsFixedOutcomesBySampleIndex(t *testing.T) {
	outcomes := []bool{true, true, false, true, true}
	runner := testrunner.NewSyntheticRunner(1.0, 99).WithFixedOutcomes(outcomes)
	sampler := NewSampler(builtinEvals())
	res, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario:    baseScenario("serial_order", len(outcomes)),
		Runner:      runner,
		Concurrency: 1,
		MinSamples:  1,
	})
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if len(res.PerRunEvidence) != len(outcomes) {
		t.Fatalf("evidence=%d want %d", len(res.PerRunEvidence), len(outcomes))
	}
	for i, want := range outcomes {
		if res.PerRunEvidence[i].Passed != want {
			t.Fatalf("sample %d passed=%v want %v", i, res.PerRunEvidence[i].Passed, want)
		}
	}
	if res.Passes != 4 {
		t.Fatalf("passes=%d want 4", res.Passes)
	}
}

func TestEachSampleHasExactlyOneTrace(t *testing.T) {
	inner := testrunner.NewSyntheticRunner(1.0, 11)
	runner := &capturingRunner{inner: inner}
	sampler := NewSampler(builtinEvals())
	const n = 20
	res, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario:    baseScenario("one_trace", n),
		Runner:      runner,
		Concurrency: 1,
		MinSamples:  1,
	})
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if res.Passes != n {
		t.Fatalf("passes=%d want %d", res.Passes, n)
	}
	if len(runner.runs) != n {
		t.Fatalf("runs=%d want %d", len(runner.runs), n)
	}
	seen := map[string]struct{}{}
	for i, run := range runner.runs {
		if run.RunID == "" {
			t.Fatalf("sample %d missing run_id", i)
		}
		if _, dup := seen[run.RunID]; dup {
			t.Fatalf("duplicate run_id %q", run.RunID)
		}
		seen[run.RunID] = struct{}{}
		if len(run.Trace) != 1 {
			t.Fatalf("sample %d trace len=%d want 1", i, len(run.Trace))
		}
	}
}

func TestHTTPOrderedFixturesConcurrentIsolation(t *testing.T) {
	provider := fixtures.NewMemoryFixtureProvider()
	if err := provider.LoadFixtures(orderedPollFixtures()); err != nil {
		t.Fatal(err)
	}
	runner := &testrunner.FixtureProbeRunner{
		Calls: []ports.ToolCall{{Name: "poll"}, {Name: "poll"}},
	}
	sc := baseScenario("http_ordered_iso", 20)
	sampler := NewSampler(builtinEvals())
	res, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario:           sc,
		Runner:             runner,
		Concurrency:        4,
		FixtureProvider:    provider,
		HasOrderedFixtures: true,
		ProxyFactory:       fixtures.StartProxy,
		MinSamples:         1,
	})
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if res.ExecutionErrors != 0 || res.Passes != 20 {
		t.Fatalf("HTTP isolation: passes=%d errors=%d", res.Passes, res.ExecutionErrors)
	}
}

func TestRetryResetsFixtureSequence(t *testing.T) {
	provider := fixtures.NewMemoryFixtureProvider()
	if err := provider.LoadFixtures(orderedPollFixtures()); err != nil {
		t.Fatal(err)
	}
	runner := &testrunner.FixtureProbeRunner{
		Calls:           []ports.ToolCall{{Name: "poll"}, {Name: "poll"}},
		FailFirst:       true,
		ConsumeThenFail: true,
	}
	sc := baseScenario("retry_fx", 4)
	sc.Assertions = []api.Assertion{{ID: "ok", Type: api.AssertTaskSuccess}}
	sampler := NewSampler(builtinEvals())
	res, err := sampler.RunScenario(context.Background(), SamplingConfig{
		Scenario:           sc,
		Runner:             runner,
		Concurrency:        4,
		FixtureProvider:    provider,
		HasOrderedFixtures: true,
		ProxyFactory:       testProxyFactory,
		MinSamples:         1,
		Retry:              api.RetryPolicy{MaxAttempts: 2, On: api.RetryOnTransient, BackoffMs: 1},
	})
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if res.Passes != 4 || res.ExecutionErrors != 0 {
		t.Fatalf("retry should reset sequence: passes=%d errors=%d", res.Passes, res.ExecutionErrors)
	}
}
