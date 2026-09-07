package validation

import (
	"context"
	"fmt"
	"sync/atomic"

	"gust/internal/adapters/fixtures"
	"gust/internal/adapters/testrunner"
	"gust/internal/core/analyze"
	"gust/internal/core/policy"
	"gust/internal/core/replay"
	coretest "gust/internal/core/test"
	"gust/internal/ports"
	"gust/pkg/api"
)

func runInvariant(ctx context.Context, evals []ports.Evaluator, eng *analyze.Engine, c Case) (bool, string, error) {
	switch c.InvariantID {
	case "s1_s8":
		return runMode3(ctx, evals, c)
	case "s2":
		return runS2NonTransient(ctx, evals)
	case "s3_s4":
		return runS3S4Traces(ctx, evals)
	case "s5":
		return runInfra(ctx, evals, c)
	case "s6", "s6_hard":
		return runAnalyze(ctx, eng, c)
	case "s7":
		return runS7EquivalentEvidence(ctx, evals)
	case "s9":
		return runS9PolicyThresholds()
	case "s10":
		return runS10ReplayNoLive(ctx)
	default:
		return false, "", fmt.Errorf("unknown invariant %q", c.InvariantID)
	}
}

type crashThenOkRunner struct {
	inner ports.TestRunner
	calls atomic.Int32
}

func (r *crashThenOkRunner) Name() string { return "crash_then_ok" }

func (r *crashThenOkRunner) Run(ctx context.Context, scenario api.TestScenario, fixtureEndpoint string) (api.AgentRun, error) {
	if r.calls.Add(1) == 1 {
		return api.AgentRun{}, fmt.Errorf("agent process crashed")
	}
	return r.inner.Run(ctx, scenario, fixtureEndpoint)
}

func runS2NonTransient(ctx context.Context, evals []ports.Evaluator) (bool, string, error) {
	inner := testrunner.NewSyntheticRunner(1.0, 1)
	runner := &crashThenOkRunner{inner: inner}
	sampler := coretest.NewSampler(evals)
	sc := api.TestScenario{
		ID:         "s2",
		Task:       api.TaskInfo{ID: "t", Input: "x"},
		Assertions: []api.Assertion{{ID: "a1", Type: api.AssertTaskSuccess}},
		Reliability: api.ReliabilityConfig{
			Samples: 1, MinimumPassRate: 0.5, Confidence: 0.95,
		},
	}
	res, err := sampler.RunScenario(ctx, coretest.SamplingConfig{
		Scenario:              sc,
		Runner:                runner,
		Concurrency:           1,
		MinSamples:            1,
		MaxExecutionErrorRate: api.Float64Ptr(1),
		Retry:                 api.RetryPolicy{MaxAttempts: 3, On: api.RetryOnTransient, BackoffMs: 1},
	})
	if err != nil {
		return false, "", err
	}
	ok := res.ExecutionErrors == 1 && res.Passes == 0 && runner.calls.Load() == 1
	return ok, fmt.Sprintf("exec_err=%d passes=%d calls=%d", res.ExecutionErrors, res.Passes, runner.calls.Load()), nil
}

type tallyRunner struct {
	inner ports.TestRunner
	ids   []string
}

func (r *tallyRunner) Name() string { return "tally" }

func (r *tallyRunner) Run(ctx context.Context, scenario api.TestScenario, fixtureEndpoint string) (api.AgentRun, error) {
	run, err := r.inner.Run(ctx, scenario, fixtureEndpoint)
	if err != nil {
		return api.AgentRun{}, err
	}
	r.ids = append(r.ids, run.RunID)
	if len(run.Trace) != 1 {
		return api.AgentRun{}, fmt.Errorf("expected one span, got %d", len(run.Trace))
	}
	return run, nil
}

func runS3S4Traces(ctx context.Context, evals []ports.Evaluator) (bool, string, error) {
	inner := testrunner.NewSyntheticRunner(1.0, 3)
	runner := &tallyRunner{inner: inner}
	const n = 12
	sampler := coretest.NewSampler(evals)
	sc := api.TestScenario{
		ID:         "s3s4",
		Task:       api.TaskInfo{ID: "t", Input: "x"},
		Assertions: []api.Assertion{{ID: "a1", Type: api.AssertTaskSuccess}},
		Reliability: api.ReliabilityConfig{
			Samples: n, MinimumPassRate: 0.5, Confidence: 0.95,
		},
	}
	res, err := sampler.RunScenario(ctx, coretest.SamplingConfig{
		Scenario:    sc,
		Runner:      runner,
		Concurrency: 1,
		MinSamples:  1,
	})
	if err != nil {
		return false, "", err
	}
	seen := map[string]struct{}{}
	for _, id := range runner.ids {
		if _, dup := seen[id]; dup {
			return false, fmt.Sprintf("duplicate run_id %s", id), nil
		}
		seen[id] = struct{}{}
	}
	ok := res.Passes == n && len(runner.ids) == n
	return ok, fmt.Sprintf("passes=%d traces=%d unique=%d", res.Passes, len(runner.ids), len(seen)), nil
}

func runS7EquivalentEvidence(ctx context.Context, evals []ports.Evaluator) (bool, string, error) {
	runOnce := func(seed int64) (*api.ReliabilityResult, error) {
		runner := testrunner.NewSyntheticRunner(1.0, seed).WithFixedOutcomes([]bool{true, true, false, true, true})
		sampler := coretest.NewSampler(evals)
		sc := api.TestScenario{
			ID:         "s7",
			Task:       api.TaskInfo{ID: "t", Input: "x"},
			Assertions: []api.Assertion{{ID: "a1", Type: api.AssertTaskSuccess}},
			Reliability: api.ReliabilityConfig{
				Samples: 5, MinimumPassRate: 0.5, Confidence: 0.95,
			},
		}
		return sampler.RunScenario(ctx, coretest.SamplingConfig{
			Scenario:    sc,
			Runner:      runner,
			Concurrency: 1,
			MinSamples:  1,
		})
	}
	a, err := runOnce(99)
	if err != nil {
		return false, "", err
	}
	b, err := runOnce(99)
	if err != nil {
		return false, "", err
	}
	ok := a.Passes == b.Passes && a.ExecutionErrors == b.ExecutionErrors && a.Verdict == b.Verdict
	if ok && len(a.PerRunEvidence) == len(b.PerRunEvidence) {
		for i := range a.PerRunEvidence {
			if a.PerRunEvidence[i].Passed != b.PerRunEvidence[i].Passed || a.PerRunEvidence[i].Score != b.PerRunEvidence[i].Score {
				ok = false
				break
			}
		}
	} else {
		ok = false
	}
	return ok, fmt.Sprintf("passes=%d/%d evidence=%d/%d", a.Passes, b.Passes, len(a.PerRunEvidence), len(b.PerRunEvidence)), nil
}

func runS9PolicyThresholds() (bool, string, error) {
	eng := policy.NewEngine()
	res := &api.ReliabilityResult{
		ScenarioID: "s9",
		Samples:    10,
		Passes:     9,
		Verdict:    api.VerdictPass,
		PerRunEvidence: []api.EvaluationResult{
			{EvaluatorName: "forbidden_tool", Passed: false},
		},
	}
	allow := api.Policy{
		Name:            "allow_one",
		HardConstraints: api.HardConstraints{ForbiddenTools: 1, SchemaViolations: 0},
		Reliability:     api.PolicyReliability{DefaultMinimumPassRate: 0.95, OnFlaky: "warn"},
	}
	deny := allow
	deny.Name = "deny"
	deny.HardConstraints.ForbiddenTools = 0
	vAllow := eng.Evaluate(allow, []*api.ReliabilityResult{res})
	vDeny := eng.Evaluate(deny, []*api.ReliabilityResult{res})
	ok := vAllow.ExitCode == api.ExitSuccess && vDeny.ExitCode == api.ExitFailure
	return ok, fmt.Sprintf("allow_exit=%d deny_exit=%d", vAllow.ExitCode, vDeny.ExitCode), nil
}

type spyFixture struct {
	inner   ports.FixtureProvider
	lookups atomic.Int32
	records atomic.Int32
}

func (s *spyFixture) Lookup(ctx context.Context, call ports.ToolCall) (api.RecordedResponse, bool, error) {
	s.lookups.Add(1)
	return s.inner.Lookup(ctx, call)
}

func (s *spyFixture) Record(ctx context.Context, call ports.ToolCall, resp api.RecordedResponse) error {
	s.records.Add(1)
	return s.inner.Record(ctx, call, resp)
}

func (s *spyFixture) Reset() error { return s.inner.Reset() }

func runS10ReplayNoLive(ctx context.Context) (bool, string, error) {
	mem := fixtures.NewMemoryFixtureProvider()
	if err := mem.LoadFixtures([]api.Fixture{{
		FixtureID:        "fx_echo",
		Tool:             "echo",
		RecordedInput:    map[string]any{"msg": "hi"},
		RecordedResponse: api.RecordedResponse{Status: "success", Body: "ok"},
		Provenance:       api.ProvenanceRecorded,
	}}); err != nil {
		return false, "", err
	}
	spy := &spyFixture{inner: mem}
	eng := replay.NewReplayEngine(spy)
	run := withTrace(baseRun("s10"), toolSpan("s1", "echo", map[string]any{"msg": "hi"}, true))
	if _, err := eng.ReplayTrace(ctx, run); err != nil {
		return false, "", err
	}
	ok := spy.lookups.Load() >= 1 && spy.records.Load() == 0
	return ok, fmt.Sprintf("lookups=%d records=%d", spy.lookups.Load(), spy.records.Load()), nil
}
