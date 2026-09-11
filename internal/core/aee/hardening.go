package aee

import (
	"context"
	"fmt"
	"sort"
	"time"

	"gust/internal/adapters/evaluators"
	"gust/internal/adapters/fixtures"
	"gust/internal/core/analyze"
	"gust/internal/core/mutate"
	"gust/internal/core/policy"
	"gust/internal/core/replay"
	"gust/internal/ports"
	"gust/pkg/api"
	"gust/pkg/jcs"
)

// HardeningExtras are issue #4 AEE proof fields (do not mix into the deterministic gate score).
type HardeningExtras struct {
	MutatorBreakdown  map[string]*mutateClass `json:"mutator_breakdown,omitempty"`
	AssertionCoverage map[string]bool         `json:"assertion_coverage,omitempty"`
	AnalyzeLatency    []TraceLatency          `json:"analyze_latency,omitempty"`
	RegressionNoiseOK bool                    `json:"regression_noise_ok"`
	ReplayIdentityOK  bool                    `json:"replay_identity_ok"`
	JudgeSpearmanNote string                  `json:"judge_spearman_note,omitempty"`
}

type mutateClass struct {
	Applied  int     `json:"applied"`
	Detected int     `json:"detected"`
	Rate     float64 `json:"rate"`
}

// TraceLatency is Analyze wall time for a synthetic trace size.
type TraceLatency struct {
	Spans   int   `json:"spans"`
	P50Ns   int64 `json:"p50_ns"`
	P95Ns   int64 `json:"p95_ns"`
	Samples int   `json:"samples"`
}

func (rep *Report) attachHardening(ctx context.Context, mutBreakdown map[string]*mutate.ClassStats) error {
	if rep.Hardening == nil {
		rep.Hardening = &HardeningExtras{}
	}
	if mutBreakdown != nil {
		rep.Hardening.MutatorBreakdown = make(map[string]*mutateClass, len(mutBreakdown))
		for k, v := range mutBreakdown {
			rep.Hardening.MutatorBreakdown[k] = &mutateClass{Applied: v.Applied, Detected: v.Detected, Rate: v.Rate}
		}
	}
	rep.Hardening.AssertionCoverage = assertionCoverage()
	rep.Hardening.AnalyzeLatency = measureAnalyzeLatency(ctx)
	rep.Hardening.RegressionNoiseOK = regressionNoiseOK()
	ok, err := replayIdentityOK(ctx)
	if err != nil {
		return err
	}
	rep.Hardening.ReplayIdentityOK = ok
	rep.Hardening.JudgeSpearmanNote = "judge calibration Spearman ρ is measured by gust judge calibrate; never mixed into the deterministic AEE gate"
	return nil
}

func assertionCoverage() map[string]bool {
	covered := map[string]bool{}
	for _, e := range evaluators.AllBuiltinEvaluators() {
		if e.Name() == "llm_judge" || e.Name() == "judge_panel" {
			continue
		}
		covered[e.Name()] = true
	}
	types := []string{
		"task_success", "tool_selection", "tool_arguments", "tool_sequence", "forbidden_tool",
		"required_tool", "max_steps", "max_latency", "error_recovery", "schema_validation",
		"agent_handoff", "role_adherence", "coordination_order",
	}
	out := map[string]bool{}
	for _, t := range types {
		out[t] = covered[t]
	}
	return out
}

func measureAnalyzeLatency(ctx context.Context) []TraceLatency {
	evals := deterministicEvaluators()
	engine := analyze.NewEngine(evals)
	sizes := []int{10, 100, 1000, 10000}
	out := make([]TraceLatency, 0, len(sizes))
	const trials = 21
	for _, n := range sizes {
		run := syntheticTrace(n)
		samples := make([]int64, 0, trials)
		for i := 0; i < trials; i++ {
			start := time.Now()
			_, _ = engine.AnalyzeRun(ctx, run, []api.Assertion{{ID: "a", Type: api.AssertTaskSuccess}}, ports.EvaluationContext{})
			samples = append(samples, time.Since(start).Nanoseconds())
		}
		sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
		out = append(out, TraceLatency{
			Spans: n, Samples: trials,
			P50Ns: samples[len(samples)/2],
			P95Ns: samples[int(float64(len(samples)-1)*0.95)],
		})
	}
	return out
}

func syntheticTrace(n int) api.AgentRun {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	trace := make([]api.Span, 0, n)
	for i := 0; i < n; i++ {
		trace = append(trace, api.Span{
			SpanID: fmt.Sprintf("s%d", i), Name: "echo", Type: api.SpanTypeTool,
			StartTime: now.Add(time.Duration(i) * time.Millisecond),
			EndTime:   now.Add(time.Duration(i+1) * time.Millisecond),
			Status:    api.SpanStatus{Code: "ok"},
			Attributes: map[string]any{"input": map[string]any{"i": i}},
		})
	}
	return api.AgentRun{
		SchemaVersion: api.SchemaVersion, RunID: fmt.Sprintf("perf_%d", n),
		Agent: api.AgentInfo{Name: "perf", Version: "1"}, Task: api.TaskInfo{ID: "t", Input: "x"},
		Trace: trace, Outcome: api.RunOutcome{Status: "completed"},
	}
}

func regressionNoiseOK() bool {
	base := policy.ExperimentStats{Passes: 95, Samples: 100, PassRate: 0.95}
	noise := policy.ExperimentStats{Passes: 94, Samples: 100, PassRate: 0.94}
	r := policy.CompareRegression(base, noise, api.PolicyRegression{MaxPassRateDrop: 0.02})
	return !r.Regressed
}

func replayIdentityOK(ctx context.Context) (bool, error) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion, RunID: "replay_id",
		Agent: api.AgentInfo{Name: "a", Version: "1"}, Task: api.TaskInfo{ID: "t", Input: "x"},
		Trace: []api.Span{{
			SpanID: "t1", Name: "echo", Type: api.SpanTypeTool, StartTime: now, EndTime: now.Add(time.Millisecond),
			Attributes: map[string]any{"input": map[string]any{"msg": "hi"}, "output": "hi"},
			Status:     api.SpanStatus{Code: "ok"},
		}},
		Outcome: api.RunOutcome{Status: "completed", Output: "hi"},
	}
	provider := fixtures.NewMemoryFixtureProvider()
	_ = provider.LoadFixtures([]api.Fixture{{
		FixtureID: "fx", Tool: "echo", MatchStrategy: api.MatchStrategyExactHash,
		RecordedInput:    map[string]any{"msg": "hi"},
		RecordedResponse: api.RecordedResponse{Status: "success", Body: "hi"},
		Provenance:       api.ProvenanceRecorded,
	}})
	eng := replay.NewReplayEngine(provider)
	a, err := eng.ReplayTrace(ctx, run)
	if err != nil {
		return false, err
	}
	b, err := eng.ReplayTrace(ctx, run)
	if err != nil {
		return false, err
	}
	ha, err := jcs.ContentHash(a)
	if err != nil {
		return false, err
	}
	hb, err := jcs.ContentHash(b)
	if err != nil {
		return false, err
	}
	return ha == hb, nil
}
