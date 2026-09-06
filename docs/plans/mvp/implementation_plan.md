# Implementation Plan: Phase 6 & Phase 7 — Mode 3 Test, Statistical Reliability & Policy Engine

Implement **Phase 6** (Mode 3 Test execution engine, Ollama/local model HTTP runner, parallel sampling, Wilson score confidence interval statistical engine, and empirical proof of Hypothesis H7) and **Phase 7** (Policy Engine evaluating hard constraints, soft reliability thresholds, configurable `on_flaky` actions, and baseline-vs-candidate regression comparison).

---

## User Review Required

> [!IMPORTANT]
> - All implementations continue to use **100% pure Go standard library** (`math`, `net/http`, `sync`, `time`, `encoding/json`).
> - The statistical engine implements the exact closed-form Wilson Score interval with boundary clamping and zero-variance handling.
> - Hypothesis H7 will be scientifically proven in unit tests using Wilson-consistent sample sizes: e.g. 100/100 → `PASS`, 20/20 → `FLAKY`, 17/20 or 40/100 → `FAIL` against a 95% threshold (N=20 can never `PASS` at P_min=0.95).

---

## Proposed Changes

### Phase 6: Mode 3 (Test) & Statistical Reliability Engine

#### [NEW] [wilson.go](file:///d:/Projects/gust/internal/core/stats/wilson.go)
- Implements:
  - `CalculateWilsonScore(passes, total int, confidence float64) (WilsonInterval, error)`
  - Edge cases: $n < 5 \to$ `INSUFFICIENT_SAMPLES`, $k=0$, $k=n$, and clamping to $[0.0, 1.0]$.
  - `ClassifyVerdict(interval, total, minPassRate, minSamples) api.VerdictType`

#### [NEW] [synthetic.go](file:///d:/Projects/gust/internal/adapters/testrunner/synthetic.go)
- Implements `ports.TestRunner`:
  - Stochastic agent simulator with configurable success probability $p$ and deterministic seed.
  - Used for scientific verification of the statistical engine without incurring model costs or latency.

#### [NEW] [ollama.go](file:///d:/Projects/gust/internal/adapters/testrunner/ollama.go)
- Implements `ports.TestRunner`:
  - Connects to local Ollama / OpenAI-compatible endpoint over HTTP (`net/http`).
  - Directs tool calls to the Ephemeral Mock Tool Proxy via `AGENTEVAL_FIXTURE_ENDPOINT`.

#### [NEW] [sampler.go](file:///d:/Projects/gust/internal/core/test/sampler.go)
- Parallel sampling orchestrator:
  - Executes $N$ independent runs with bounded worker pools.
  - Resolves assertions for each run using registered evaluators.
  - Computes Wilson score interval and emits `api.ReliabilityResult`.

---

### Phase 7: Policy Engine & Regression Comparator

#### [NEW] [policy/engine.go](file:///d:/Projects/gust/internal/core/policy/engine.go)
- Evaluates declared `api.Policy`:
  - Enforces Hard Constraints (e.g. `forbidden_tools: 0`, `schema_violations: 0`) across all per-run evidence.
  - Evaluates Reliability verdicts (`PASS`, `FAIL`, `FLAKY`).
  - Enforces `on_flaky`: `warn` (CI pass, exit code 0), `fail` (CI fail, exit code 3), `ignore` (pass, exit code 0).

#### [NEW] [policy/regression.go](file:///d:/Projects/gust/internal/core/policy/regression.go)
- Baseline vs. Candidate regression comparator:
  - Compares pass rates and latency between two experiments.
  - Flags statistically significant drops exceeding policy thresholds.

---

## Verification Plan

### Automated Tests
1. **Wilson Score Statistical Tests**:
   - `go test -v ./internal/core/stats/...`
   - Test $k=0$, $k=n$, sample sizes $N=5$ to $N=100$, confidence levels 90%, 95%, 99%.
2. **Proof of Hypothesis H7**:
   - `go test -v ./internal/core/test/...`
   - Wilson-consistent vectors against $95\%$ threshold:
     - $100/100$ or synthetic $p \approx 1.0$, $N=100$ → `PASS`
     - $20/20$ or synthetic $p = 1.0$, $N=20$ → `FLAKY`
     - $17/20$ or synthetic $p = 0.40$, $N=20$ → `FAIL`
3. **Policy Engine Tests**:
   - `go test -v ./internal/core/policy/...`
   - Verify `on_flaky: warn` vs `on_flaky: fail` produces exit code 0 vs 3 on the same input.
   - Verify single hard constraint violation immediately fails policy.
   - Verify regression detection between baseline and candidate.
