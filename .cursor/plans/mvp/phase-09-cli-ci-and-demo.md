# Phase 9: CLI, CI Integration, Demo & Verification

## 1. Objectives & Scope
1. Implement the unified **CLI binary (`gust`)** using Cobra, distributed as a single static Go binary.
2. Implement rich terminal formatters (ANSI color tables, clear `FLAKY` indicators) and CI-optimized outputs (GitHub Actions Step Summaries, JSON output).
3. Enforce deterministic exit codes for automated CI pipelines.
4. Execute and verify the **MVP demo** end-to-end against a $0 local model / mock environment.
5. Complete the entire **MVP Definition of Done**.

---

## 2. Package Architecture

```text
gust/
├── cmd/
│   └── gust/
│       └── main.go              # Entrypoint
├── internal/
│   └── cli/
│       ├── root.go              # Root Cobra command & global flags
│       ├── analyze.go           # 'gust analyze'
│       ├── replay.go            # 'gust replay'
│       ├── test.go              # 'gust test'
│       ├── mutate.go            # 'gust mutate'
│       ├── compare.go           # 'gust compare'
│       ├── scenario.go          # 'gust scenario from-run'
│       ├── formatters/          # Output renderers
│       │   ├── terminal.go      # ANSI tables & colored badges
│       │   ├── github.go        # GitHub Actions annotations & summaries
│       │   └── json.go          # Structured JSON stream
│       └── cli_test.go
```

---

## 3. CLI Commands & Flag Specifications

```bash
# Mode 1: Analyze
gust analyze <run.json> --policy <policy.yaml> [--json]

# Mode 2: Replay
gust replay <run.json> --fixtures <fixture_dir> [--json]

# Mode 3: Test
gust test <scenario.yaml> --samples 20 --concurrency 4 --runner ollama --endpoint http://localhost:11434 [--policy policy.yaml] [--json]

# Mutation Engine
gust mutate <agent.json> --classes all --n 100 [--json]

# Baseline vs. Candidate Regression Compare
gust compare <baseline.json> <candidate.json> --policy <policy.yaml> [--json]

# Assisted Scenario Extraction
gust scenario from-run <run.json> --output <scenario.yaml>
```

---

## 4. Output Formatting & Distinct `FLAKY` Rendering

A critical requirement of the spec is that **`FLAKY` must render distinctly from `FAIL` or `PASS`** across all formats.

### 4.1 ANSI Terminal Formatting
```text
$ gust test cancel_latest_order.yaml --samples 20

Running cancel_latest_order against support-agent:1.4 (model: llama3.1:8b)

  17/20 passed  (observed pass rate: 85.0%)
  95% confidence interval: [64.0%, 95.0%]
  required minimum pass rate: 95.0%

VERDICT: [?] FLAKY (Inconclusive)
  The agent is not reliably calling cancel_order(123) instead of cancel_order(122).
  3 of 20 runs called the wrong order. See --verbose for per-run evidence.
```

### 4.2 GitHub Actions Summary Output
Emits `$GITHUB_STEP_SUMMARY` markdown:
```markdown
## gust Test Results

| Scenario | Samples | Pass Rate | 95% CI | Verdict |
|---|---|---|---|---|
| `cancel_latest_order` | 20 | 85.0% | `[64.0%, 95.0%]` | ⚠️ **FLAKY** |
| `refund_processed` | 20 | 100.0% | `[83.2%, 100.0%]` | ✅ **PASS** |
```

### 4.3 CI Exit Codes
- `0`: Success (All tests passed, or `FLAKY` under `on_flaky: warn` or `ignore`).
- `1`: Regression or Failure (Scenario failed minimum pass rate, or hard constraint violated).
- `2`: System Error (Bad CLI flags, invalid file path, unparseable JSON/YAML).
- `3`: Flaky Failure (Inconclusive test result under `on_flaky: fail`).

---

## 5. End-to-End Demo Verification

To declare the MVP complete, run the three demo workflows:

```bash
# 1. Mutation Testing
$ gust mutate testdata/golden_agent.json --classes all --n 100
# Expected: >= 90% Detection Rate, <= 5% FPR, median eval <= 5ms

# 2. Probabilistic Testing
$ gust test testdata/scenarios/cancel_latest_order.yaml --samples 20
# Expected: Correctly identifies flaky synthetic/local agent and computes Wilson interval

# 3. Regression Comparison
$ gust compare testdata/baseline.json testdata/candidate.json
# Expected: Flags statistically significant regression with policy citation
```

---

## 6. Definition of Done (MVP Verification Checklist)

- [x] `AgentRun`, `Span`, `Fixture`, `TestScenario`, `Assertion`, `Policy`, `EvaluationResult`, and `ReliabilityResult` schemas ship as Draft 2020-12 contracts under `spec/schemas/` (runtime uses hand-rolled `Validate()`).
- [x] RFC 8785 Canonical JSON hashing passes project JCS test vectors (`pkg/jcs`).
- [x] Tier 1 (Go) and Tier 2 (stdio JSON-RPC) interfaces are operational and tested.
- [x] All 10 built-in deterministic evaluators pass throughput gate ($\ge 1,000$ evals/sec).
- [x] Mode 1 (Analyze) and Mode 2 (Replay) operate 100% offline with zero external network traffic.
- [x] Replay determinism test (100 repeated runs) produces identical content hashes.
- [x] Mutation engine achieves $\ge 90\%$ detection rate and $\le 5\%$ FPR.
- [x] Ephemeral Tool Mock Proxy server intercepts tool calls and injects simulated failures.
- [x] Mode 3 (Test) runs parallel sampling using bounded worker pools (synthetic always; Ollama when available).
- [x] Wilson score statistical engine correctly classifies Wilson-consistent H7 vectors (`PASS`/`FLAKY`/`FAIL`).
- [x] Scenario extraction structurally avoids populating assertions from trace behavior (Hypothesis H8).
- [x] Policy Engine evaluates hard and soft constraints, supporting all `on_flaky` modes.
- [x] Single static binary builds (`go build -o gust ./cmd/gust`); demo scripts under `demo/`.
- [x] Apache-2.0 `LICENSE` and CI workflow (`.github/workflows/ci.yml`).
