package testrunner

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

var _ ports.TestRunner = (*SyntheticRunner)(nil)

// SyntheticRunner emits stochastic AgentRuns with configurable success probability.
// Used for Hypothesis H7 proofs and CI without model cost.
type SyntheticRunner struct {
	PassProbability float64
	Seed            int64
	AgentName       string
	AgentVersion    string

	mu    sync.Mutex
	rng   *rand.Rand
	seq   atomic.Uint64
	fixed []bool // optional scripted outcomes; when set, overrides probability
}

// NewSyntheticRunner creates a seeded Bernoulli test runner.
func NewSyntheticRunner(passProbability float64, seed int64) *SyntheticRunner {
	if passProbability < 0 {
		passProbability = 0
	}
	if passProbability > 1 {
		passProbability = 1
	}
	return &SyntheticRunner{
		PassProbability: passProbability,
		Seed:            seed,
		AgentName:       "synthetic-agent",
		AgentVersion:    "1.0",
		rng:             rand.New(rand.NewSource(seed)),
	}
}

// WithFixedOutcomes configures a deterministic pass/fail sequence (cycles if exhausted).
func (r *SyntheticRunner) WithFixedOutcomes(outcomes []bool) *SyntheticRunner {
	r.fixed = append([]bool(nil), outcomes...)
	return r
}

func (r *SyntheticRunner) Name() string { return "synthetic" }

func (r *SyntheticRunner) Run(ctx context.Context, scenario api.TestScenario, fixtureEndpoint string) (api.AgentRun, error) {
	select {
	case <-ctx.Done():
		return api.AgentRun{}, ctx.Err()
	default:
	}

	n := r.seq.Add(1)
	passed := r.nextOutcome()

	status := "completed"
	output := "ok"
	errMsg := ""
	if !passed {
		status = "failed"
		output = ""
		errMsg = "synthetic failure"
	}

	now := time.Now().UTC()
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         fmt.Sprintf("synthetic_%s_%d", scenario.ID, n),
		Agent: api.AgentInfo{
			Name:    r.AgentName,
			Version: r.AgentVersion,
		},
		Task: scenario.Task,
		Trace: []api.Span{{
			SpanID:    fmt.Sprintf("span_%d", n),
			Name:      "synthetic_step",
			Type:      api.SpanTypeAgent,
			StartTime: now,
			EndTime:   now,
			Status:    api.SpanStatus{Code: "ok"},
		}},
		Outcome: api.RunOutcome{
			Status:     status,
			Output:     output,
			Error:      errMsg,
			DurationNs: time.Millisecond,
		},
		Metadata: map[string]any{
			"fixture_endpoint": fixtureEndpoint,
			"runner":           r.Name(),
		},
	}
	return run, nil
}

func (r *SyntheticRunner) nextOutcome() bool {
	if len(r.fixed) > 0 {
		idx := int(r.seq.Load()-1) % len(r.fixed)
		return r.fixed[idx]
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rng == nil {
		r.rng = rand.New(rand.NewSource(r.Seed))
	}
	return r.rng.Float64() < r.PassProbability
}
