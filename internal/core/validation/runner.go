package validation

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"gust/internal/adapters/evaluators"
	"gust/internal/adapters/fixtures"
	"gust/internal/adapters/mutators"
	"gust/internal/adapters/testrunner"
	"gust/internal/core/analyze"
	"gust/internal/core/mutate"
	"gust/internal/core/stats"
	coretest "gust/internal/core/test"
	"gust/internal/ports"
	"gust/pkg/api"
)

// Run executes every case and returns a published-style report.
func Run(ctx context.Context) (*Report, error) {
	cases := AllCases()
	rep := &Report{
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		SuiteVersion: SuiteVersion,
		Total:        len(cases),
		ByCategory:   map[string]CatStats{},
		Results:      make([]CaseResult, 0, len(cases)),
	}

	detEvals := deterministicEvaluators()
	analyzeEng := analyze.NewEngine(detEvals)
	goldens := mutate.BuildGoldenSuite()
	mutByName := map[string]ports.Mutator{}
	for _, m := range mutators.AllBuiltinMutators() {
		mutByName[m.Name()] = m
	}

	for _, c := range cases {
		cr := CaseResult{ID: c.ID, Category: c.Category, Question: c.Question, Kind: c.Kind}
		var err error
		switch c.Kind {
		case KindAnalyze:
			cr.Passed, cr.Detail, err = runAnalyze(ctx, analyzeEng, c)
		case KindWilson:
			cr.Passed, cr.Detail, err = runWilson(c)
		case KindInfra:
			cr.Passed, cr.Detail, err = runInfra(ctx, detEvals, c)
		case KindMutation:
			cr.Passed, cr.Detail, err = runMutation(ctx, analyzeEng, goldens, mutByName, c)
		case KindFixture:
			cr.Passed, cr.Detail, err = runFixture(ctx, c)
		case KindMode3:
			cr.Passed, cr.Detail, err = runMode3(ctx, detEvals, c)
		case KindInvariant:
			cr.Passed, cr.Detail, err = runInvariant(ctx, detEvals, analyzeEng, c)
		default:
			err = fmt.Errorf("unknown kind %q", c.Kind)
		}
		if err != nil {
			cr.Passed = false
			cr.Detail = err.Error()
		}
		rep.Results = append(rep.Results, cr)
		st := rep.ByCategory[string(c.Category)]
		st.Total++
		if cr.Passed {
			st.Passed++
			rep.Passed++
		} else {
			rep.Failed++
			rep.Failures = append(rep.Failures, cr)
		}
		if st.Total > 0 {
			st.Rate = float64(st.Passed) / float64(st.Total)
		}
		rep.ByCategory[string(c.Category)] = st
	}

	if rep.Total > 0 {
		rep.PassRate = float64(rep.Passed) / float64(rep.Total)
	}
	rep.GatePassed = rep.Failed == 0 && rep.PassRate >= MinPassRate
	return rep, nil
}

func deterministicEvaluators() []ports.Evaluator {
	all := evaluators.AllBuiltinEvaluators()
	out := make([]ports.Evaluator, 0, len(all))
	for _, e := range all {
		if e.Name() == "llm_judge" {
			continue
		}
		out = append(out, e)
	}
	return out
}

func runAnalyze(ctx context.Context, eng *analyze.Engine, c Case) (bool, string, error) {
	report, err := eng.AnalyzeRun(ctx, c.Run, c.Assertions, ports.EvaluationContext{})
	if err != nil {
		return false, "", err
	}
	ok := report.Passed == c.WantPassed
	detail := fmt.Sprintf("analyze.passed=%v want=%v", report.Passed, c.WantPassed)
	return ok, detail, nil
}

func runWilson(c Case) (bool, string, error) {
	conf := c.Confidence
	if conf <= 0 {
		conf = 0.95
	}
	minS := c.MinSamples
	if minS <= 0 {
		minS = 5
	}
	iv, err := stats.CalculateWilsonScore(c.Passes, c.Total, conf)
	if err != nil {
		return false, "", err
	}
	got := stats.ClassifyVerdict(iv, c.Total, c.MinPass, minS)
	ok := got == c.WantVerdict
	return ok, fmt.Sprintf("verdict=%s want=%s ci=[%.3f,%.3f]", got, c.WantVerdict, iv.LowerBound, iv.UpperBound), nil
}

type errOnceRunner struct {
	inner      ports.TestRunner
	failFirstN int
	calls      atomic.Int32
}

func (r *errOnceRunner) Name() string { return "validation_err_once" }

func (r *errOnceRunner) Run(ctx context.Context, req ports.SampleRequest) (api.AgentRun, error) {
	n := int(r.calls.Add(1))
	if n <= r.failFirstN {
		return api.AgentRun{}, fmt.Errorf("transient sample %d", n)
	}
	return r.inner.Run(ctx, req)
}

func runInfra(ctx context.Context, evals []ports.Evaluator, c Case) (bool, string, error) {
	inner := testrunner.NewSyntheticRunner(1.0, 1)
	runner := &errOnceRunner{inner: inner, failFirstN: c.FailFirstN}
	sampler := coretest.NewSampler(evals)
	sc := api.TestScenario{
		ID:   "infra_" + c.ID,
		Task: api.TaskInfo{ID: "t", Input: "x"},
		Assertions: []api.Assertion{{
			ID: "a1", Type: api.AssertTaskSuccess,
		}},
		Reliability: api.ReliabilityConfig{
			Samples:         c.Samples,
			MinimumPassRate: 0.5,
			Confidence:      0.95,
		},
	}
	res, err := sampler.RunScenario(ctx, coretest.SamplingConfig{
		Scenario:              sc,
		Runner:                runner,
		Concurrency:           1,
		MaxExecutionErrorRate: api.Float64Ptr(c.MaxExecRate),
		MinSamples:            1,
		Retry:                 api.RetryPolicy{MaxAttempts: 1, On: api.RetryOnNone},
	})
	if c.WantUnstable {
		if err == nil || !errors.Is(err, coretest.ErrRunnerUnstable) {
			return false, fmt.Sprintf("want ErrRunnerUnstable, got err=%v", err), nil
		}
		return true, "runner unstable as expected", nil
	}
	if err != nil {
		return false, "", err
	}
	ok := res.ExecutionErrors == c.WantExecErrs
	return ok, fmt.Sprintf("exec_errors=%d want=%d passes=%d", res.ExecutionErrors, c.WantExecErrs, res.Passes), nil
}

func runMutation(ctx context.Context, eng *analyze.Engine, goldens []mutate.GoldenCase, mutByName map[string]ports.Mutator, c Case) (bool, string, error) {
	if c.GoldenIdx < 0 || c.GoldenIdx >= len(goldens) {
		return false, "", fmt.Errorf("golden idx %d out of range", c.GoldenIdx)
	}
	m, ok := mutByName[c.MutatorName]
	if !ok {
		return false, "", fmt.Errorf("unknown mutator %q", c.MutatorName)
	}
	gc := goldens[c.GoldenIdx]
	outcome, err := m.Mutate(ctx, gc.Run)
	if err != nil {
		return false, "", err
	}
	if outcome.Status == api.MutationSkipped {
		return true, "mutator skipped (acceptable)", nil
	}
	if outcome.Status != api.MutationApplied {
		return false, fmt.Sprintf("unexpected mutation status %s", outcome.Status), nil
	}
	report, err := eng.AnalyzeRun(ctx, outcome.MutatedRun, gc.Assertions, ports.EvaluationContext{})
	if err != nil {
		return false, "", err
	}
	detected := !report.Passed
	ok = detected == c.WantDetected
	return ok, fmt.Sprintf("detected=%v want=%v", detected, c.WantDetected), nil
}

func runFixture(ctx context.Context, c Case) (bool, string, error) {
	p := fixtures.NewMemoryFixtureProvider()
	if err := p.LoadFixtures(c.Fixtures); err != nil {
		return false, "", err
	}
	if c.ID == "fix_clone_isolates_sequence" {
		clone := p.Clone()
		_, _, _ = p.Lookup(ctx, ports.ToolCall{Name: "poll"})
		resp, found, err := clone.Lookup(ctx, ports.ToolCall{Name: "poll"})
		if err != nil {
			return false, "", err
		}
		ok := found && fmt.Sprint(resp.Body) == "PENDING"
		return ok, fmt.Sprintf("clone_found=%v body=%v", found, resp.Body), nil
	}
	calls := c.CallSequence
	if len(calls) == 0 {
		calls = []ports.ToolCall{c.Call}
	}
	var lastFound bool
	var lastStatus string
	bodies := make([]string, 0, len(calls))
	for _, call := range calls {
		resp, found, err := p.Lookup(ctx, call)
		if err != nil {
			return false, "", err
		}
		lastFound = found
		lastStatus = resp.Status
		if found {
			bodies = append(bodies, fmt.Sprint(resp.Body))
		} else {
			bodies = append(bodies, "")
		}
	}
	if len(c.WantBodies) > 0 {
		if len(bodies) < len(c.WantBodies) {
			return false, fmt.Sprintf("got %d bodies want %d", len(bodies), len(c.WantBodies)), nil
		}
		for i, want := range c.WantBodies {
			if bodies[i] != want {
				return false, fmt.Sprintf("body[%d]=%q want=%q", i, bodies[i], want), nil
			}
		}
		return true, fmt.Sprintf("bodies=%v", bodies), nil
	}
	if lastFound != c.WantFound {
		return false, fmt.Sprintf("found=%v want=%v", lastFound, c.WantFound), nil
	}
	if c.WantFound && c.WantStatus != "" && lastStatus != c.WantStatus {
		return false, fmt.Sprintf("status=%q want=%q", lastStatus, c.WantStatus), nil
	}
	return true, fmt.Sprintf("found=%v status=%s", lastFound, lastStatus), nil
}

func runMode3(ctx context.Context, evals []ports.Evaluator, c Case) (bool, string, error) {
	provider := fixtures.NewMemoryFixtureProvider()
	if err := provider.LoadFixtures(c.Fixtures); err != nil {
		return false, "", err
	}
	runner := &testrunner.FixtureProbeRunner{
		Calls:           c.Mode3Calls,
		ExtraCalls:      c.Mode3ExtraCalls,
		FailFirst:       c.Mode3FailFirst,
		ConsumeThenFail: c.Mode3FailFirst,
	}
	samples := c.Mode3Samples
	if samples <= 0 {
		samples = 8
	}
	conc := c.Mode3Concurrency
	if conc <= 0 {
		conc = 4
	}
	sc := api.TestScenario{
		ID:   "mode3_" + c.ID,
		Task: api.TaskInfo{ID: "t", Input: "x"},
		Assertions: []api.Assertion{{
			ID: "a1", Type: api.AssertTaskSuccess,
		}},
		Reliability: api.ReliabilityConfig{
			Samples:         samples,
			MinimumPassRate: 0.5,
			Confidence:      0.95,
		},
	}
	retry := api.RetryPolicy{MaxAttempts: 1, On: api.RetryOnNone}
	if c.Mode3FailFirst {
		retry = api.RetryPolicy{MaxAttempts: 2, On: api.RetryOnTransient, BackoffMs: 1}
	}
	sampler := coretest.NewSampler(evals)
	res, err := sampler.RunScenario(ctx, coretest.SamplingConfig{
		Scenario:              sc,
		Runner:                runner,
		Concurrency:           conc,
		FixtureProvider:       provider,
		HasOrderedFixtures:    fixtures.HasOrderedFixtures(c.Fixtures),
		ProxyFactory:          fixtures.StartProxy,
		MinSamples:            1,
		MaxExecutionErrorRate: api.Float64Ptr(1),
		Retry:                 retry,
	})
	if err != nil {
		return false, "", err
	}
	ok := res.Passes == c.WantMode3Passes && res.ExecutionErrors == c.WantMode3ExecErrs
	return ok, fmt.Sprintf("passes=%d want=%d exec_err=%d", res.Passes, c.WantMode3Passes, res.ExecutionErrors), nil
}
