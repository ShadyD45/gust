# Phase 6: Mode 3 (Test) & Statistical Reliability Engine

## 1. Objectives & Scope
1. Implement **Mode 3 (Test)**: Running a real agent with a real LLM against a controlled tool environment.
2. Implement **`TestRunner` backends**:
   - `OllamaRunner`: Connects to a $0 local model (Ollama / llama.cpp) over HTTP.
   - `SyntheticRunner`: Emits stochastic behaviors with configurable probability for statistical unit testing.
3. Build a concurrent **Parallel Sampling Engine** executing $N$ independent runs with bounded worker pools.
4. Implement the **Wilson Score Interval Statistical Engine** with edge-case handling ($n < 5$, $k=0$, $k=n$, clamping).
5. Empirically prove **Hypothesis H7**: Validate Wilson-consistent classification against a 95% threshold — e.g. near-perfect agents at N≥100 → `PASS`, perfect small samples (20/20) → `FLAKY`, low rates (17/20 or 40/100) → `FAIL`. Note: N=20 can never `PASS` at P_min=0.95.

---

## 2. Package Architecture

```text
gust/
├── internal/
│   ├── adapters/
│   │   └── testrunner/
│   │       ├── ollama.go        # Local Ollama / OpenAI-compatible HTTP runner
│   │       ├── synthetic.go     # Stochastic synthetic runner for statistical testing
│   │       └── testrunner_test.go
│   └── core/
│       ├── test/                # Mode 3 Parallel Sampling Orchestrator
│       │   ├── sampler.go
│       │   └── sampler_test.go
│       └── stats/               # Closed-form statistical routines
│           ├── wilson.go        # Wilson Score Interval with boundary handling
│           └── wilson_test.go
```

---

## 3. Statistical Engine & Wilson Score Formula (`internal/core/stats/wilson.go`)

### 3.1 Closed-Form Formula
Given $n$ samples, $k$ passes, and normal quantile $z$ (for $95\%$ confidence, $z = 1.95996$):

$$p_{center} = \frac{k + \frac{z^2}{2}}{n + z^2}$$
$$p_{margin} = \frac{z}{n + z^2} \sqrt{\frac{k(n - k)}{n} + \frac{z^2}{4}}$$
$$p_{lower} = \max\left(0.0, \; p_{center} - p_{margin}\right)$$
$$p_{upper} = \min\left(1.0, \; p_{center} + p_{margin}\right)$$

### 3.2 Implementation
```go
package stats

import (
    "math"
    "gust/pkg/api"
)

type WilsonInterval struct {
    LowerBound       float64 `json:"lower_bound"`
    UpperBound       float64 `json:"upper_bound"`
    ObservedPassRate float64 `json:"observed_pass_rate"`
}

func CalculateWilsonScore(passes, total int, confidence float64) (WilsonInterval, error) {
    if total <= 0 {
        return WilsonInterval{}, ErrZeroSamples
    }

    z := 1.95996 // Default 95% confidence
    if confidence == 0.99 {
        z = 2.57583
    } else if confidence == 0.90 {
        z = 1.64485
    }

    n := float64(total)
    k := float64(passes)
    p := k / n
    z2 := z * z

    center := (k + z2/2.0) / (n + z2)
    margin := (z / (n + z2)) * math.Sqrt((k*(n-k)/n) + (z2/4.0))

    lower := math.Max(0.0, center-margin)
    upper := math.Min(1.0, center+margin)

    return WilsonInterval{
        LowerBound:       lower,
        UpperBound:       upper,
        ObservedPassRate: p,
    }, nil
}
```

### 3.3 Verdict Classification Logic
```go
func ClassifyVerdict(interval WilsonInterval, total int, minPassRate float64, minSamples int) api.ReliabilityVerdict {
    if total < minSamples {
        return api.VerdictInsufficientSamples
    }
    if interval.LowerBound >= minPassRate {
        return api.VerdictPass
    }
    if interval.UpperBound < minPassRate {
        return api.VerdictFail
    }
    return api.VerdictFlaky
}
```

---

## 4. Parallel Sampling Orchestrator (`internal/core/test/sampler.go`)

```go
package test

import (
    "context"
    "sync"
    "golang.org/x/sync/errgroup"
    "gust/internal/ports"
    "gust/pkg/api"
)

type SamplingConfig struct {
    Scenario    api.TestScenario
    Runner      ports.TestRunner
    Evaluators  []ports.Evaluator
    Concurrency int
    Endpoint    string
}

type ScenarioResult struct {
    ScenarioID        string                  `json:"scenario_id"`
    Samples           int                     `json:"samples"`
    Passes            int                     `json:"passes"`
    ObservedPassRate  float64                 `json:"observed_pass_rate"`
    ConfidenceInterval [2]float64             `json:"confidence_interval"`
    Verdict           api.ReliabilityVerdict  `json:"verdict"`
    PerRunEvidence    []ports.EvaluationResult `json:"per_run_evidence"`
}

func (s *Sampler) RunScenario(ctx context.Context, cfg SamplingConfig) (*ScenarioResult, error) {
    g, ctx := errgroup.WithContext(ctx)
    g.SetLimit(cfg.Concurrency)

    results := make([]ports.EvaluationResult, cfg.Scenario.Reliability.Samples)
    passes := 0
    var mu sync.Mutex

    for i := 0; i < cfg.Scenario.Reliability.Samples; i++ {
        idx := i
        g.Go(func() error {
            run, err := cfg.Runner.Run(ctx, cfg.Scenario, cfg.Endpoint)
            if err != nil {
                return err
            }
            evalResult, err := s.evaluateRun(ctx, run, cfg.Scenario.Assertions, cfg.Evaluators)
            if err != nil {
                return err
            }

            mu.Lock()
            results[idx] = evalResult
            if evalResult.Passed {
                passes++
            }
            mu.Unlock()
            return nil
        })
    }

    if err := g.Wait(); err != nil {
        return nil, err
    }

    // Compute Wilson Score interval and verdict
    ...
}
```

---

## 5. Verification & Test Plan

- **Statistical Correctness Tests (`internal/core/stats/wilson_test.go`):**
  - Verify $k=0$ yields $p_{lower} = 0.0$ and positive upper bound.
  - Verify $k=n$ yields $p_{upper} = 1.0$ and positive lower bound.
  - Verify interval widths shrink as sample size increases from $N=5$ to $N=100$.
- **Validation of Hypothesis H7:**
  - Deterministic counts: 100/100 → `PASS`, 20/20 → `FLAKY`, 17/20 → `FAIL` at P_min=0.95.
  - `SyntheticRunner` with $p \approx 1.0$, $N=100$ → `PASS`; $p=1.0$, $N=20$ → `FLAKY`; $p=0.40$, $N=20$ → `FAIL`.
- **Local LLM Integration Test:**
  - Execute live test run against local Ollama instance running `llama3.1:8b` (or simulated response) and verify end-to-end trace capture.
