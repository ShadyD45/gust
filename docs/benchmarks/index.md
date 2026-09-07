---
title: Benchmarks
nav_order: 3
has_children: true
has_toc: false
---
# Benchmarks

gust publishes a **self-benchmark** of its own evaluation suite — Agent Evaluation Effectiveness (AEE). These numbers prove the *test infrastructure* works: it catches injected faults, stays quiet on clean runs, finishes fast enough for CI, and reproduces bit-identically.

{: .important }
AEE scores **gust**, not your agent. Agent pass rates belong to Mode 3 and your scenarios. AEE measures behavior gates, fixtures, mutation detection, and Wilson verdicts — the effectiveness of gust as test infrastructure.

## Why this is the proof that matters

A captured **trace** or a **final-answer score** alone cannot tell you whether your assertions would catch a real bug. Those signals collapse three different jobs into one number.

gust separates the jobs and measures each with the right instrument:

| Claim | How we prove it |
|-------|-----------------|
| Assertions catch real failures | **Mutation detection rate** on a golden suite |
| Assertions do not cry wolf | **False positive rate** on unmutated runs |
| Offline gates fit CI budgets | **Evaluator throughput** (deterministic path only) |
| Analyze/Replay are trustworthy | **Reproducibility** (JCS hash stability) |
| Stochastic agents get honest verdicts | **H7** Wilson vectors (`PASS` / `FLAKY` / `FAIL`) |
| Gust cannot be easily fooled | **[Validation Suite]({% link benchmarks/validation.md %})** — adversarial pass/fail/flaky/infra/mutation/fixture/recovery cases |

That is gust’s job: **test infrastructure for agents**. See [Metrics explained]({% link benchmarks/metrics.md %}) for definitions, and [Latest results]({% link benchmarks/results.md %}) for the published numbers (also [benchmark workflow runs](https://github.com/ShadyD45/gust/actions/workflows/benchmark.yml)).

## Latest snapshot

| Metric | Gate | What a pass means |
|--------|------|-------------------|
| Detection rate | ≥ 90% | Mutated bad behavior is caught |
| False positive rate | ≤ 5% | Clean runs stay green |
| Eval throughput | ≥ 1,000 cases/sec | Deterministic suite is CI-cheap |
| Reproducibility | 100% identical | Same inputs → same analyze hash |
| H7 reliability | all correct | Wilson classification matches theory |

Full tables and regenerate commands: [Latest results]({% link benchmarks/results.md %}).

## Pages in this section

| Page | What it covers |
|------|----------------|
| [Metrics explained]({% link benchmarks/metrics.md %}) | Meaning, method, and value of each AEE input |
| [Latest results]({% link benchmarks/results.md %}) | Published numbers from `gust-aee report` (updated locally; see [workflow runs](https://github.com/ShadyD45/gust/actions/workflows/benchmark.yml)) |
| [More proof]({% link benchmarks/more-proof.md %}) | Additional signals beyond the AEE table |

Reproduce anytime:

```bash
go build -o gust-aee ./cmd/gust-aee
./gust-aee report
./gust-aee report --doc benchmarks/RESULTS.md --site-doc docs/benchmarks/results.md
```
