---
title: Metrics explained
nav_order: 1
parent: Benchmarks
---
# Metrics explained

Each AEE metric answers one question about gust’s **deterministic evaluation suite** (Analyze / Replay path). LLM judge latency and agent pass rates are excluded on purpose.

Composite `aee_score` is a documented weighted blend in `internal/core/aee/report.go`. CI gates on the boolean `passed` field, not the score alone.

## Detection rate (DR)

**Question:** If we inject a known fault into a captured run, do the assertions fail?

**Method:** The mutation engine applies built-in mutators to `mutate.BuildGoldenSuite()`, then re-analyzes. A mutant counts as *detected* when at least one assertion fails after a successful mutation. Inapplicable mutants are `skipped` and do not inflate the rate.

**Gate:** ≥ 90%

**What it provides:** Trust that your evaluator suite is not a rubber stamp. High DR is the mutation-testing analogue of “these unit tests would fail if the code were wrong.”

## False positive rate (FPR)

**Question:** Do clean (unmutated) golden runs still pass?

**Method:** Same golden suite, no mutants. Any failure on a known-good run is a false positive.

**Gate:** ≤ 5%

**What it provides:** CI signal quality. A suite that fails on good behavior trains teams to ignore the gate.

## Eval throughput

**Question:** Is the deterministic path fast enough for every PR?

**Method:** Repeated evaluation of built-in deterministic evaluators only (`llm_judge` excluded). Reported as cases per second over a minimum wall-clock window.

**Gate:** ≥ 1,000 cases/sec

**What it provides:** Evidence that Analyze/Replay can sit in CI without burning minutes or dollars on LLM calls. Mode 3 agent sampling is a separate budget owned by the job that runs your agent.

## Reproducibility

**Question:** Same run + same assertions → same result?

**Method:** Analyze the golden case 20 times, content-hash a timing-stripped view of the report (RFC 8785 / JCS). Fraction of trials matching the first hash.

**Gate:** 100% identical

**What it provides:** Proof that offline modes are bit-stable — a prerequisite for mutation scores, regression compare, and trusting a red CI log.

## H7 — reliability engine

**Question:** Does the Wilson classifier label the textbook sample vectors correctly?

**Method:** Closed-form Wilson score intervals at 95% confidence, `P_min = 0.95`:

| Vector | Expected verdict | Why |
|--------|------------------|-----|
| 100/100 | `PASS` | Lower bound ≥ 0.95 |
| 20/20 | `FLAKY` | Interval straddles 0.95 (N=20 can never `PASS` at this threshold) |
| 17/20 | `FAIL` | Upper bound &lt; 0.95 |

**Gate:** all three correct

**What it provides:** Scientific backing for Mode 3’s `PASS` / `FAIL` / `FLAKY` / `INSUFFICIENT_SAMPLES` language. Without this, a single lucky run looks like a green build.

## What AEE deliberately excludes

| Excluded | Why |
|----------|-----|
| Mode 3 agent pass rates | Property of *your* agent and scenarios, not gust |
| LLM judge latency / agreement | Optional soft signal; calibrate separately |

Deeper stats design: [Statistics and policy]({% link architecture/statistics-and-policy.md %}). Reproduce commands and gates: [AEE methodology]({% link usage/aee-methodology.md %}).
