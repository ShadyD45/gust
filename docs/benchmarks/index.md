---
title: Benchmarks
nav_order: 3
has_children: true
has_toc: false
---
# Benchmarks

gust publishes a **self-benchmark** of its own evaluation suite — Agent Evaluation Effectiveness (AEE). These numbers are regression evidence that the *deterministic evaluation machinery* behaves as encoded in the golden suite: it catches injected faults on that suite, stays quiet on those clean runs, finishes fast enough for CI, and reproduces bit-identically. They are not a field estimate of Gust on arbitrary real agents.

{: .important }
AEE scores **gust**, not your agent. Agent pass rates belong to Mode 3 and your scenarios. AEE measures behavior gates, fixtures, mutation detection, and Wilson verdicts — the effectiveness of gust as test infrastructure.

## Why this is the proof that matters

A captured **trace** or a **final-answer score** alone cannot tell you whether your assertions would catch a real bug. Those signals collapse three different jobs into one number.

gust separates the jobs and measures each with the right instrument:

| Claim | How we measure it |
|-------|-----------------|
| Assertions catch injected faults on the golden suite | **Mutation detection rate** |
| Assertions do not fail those clean golden runs | **Golden-suite false positive rate** |
| Offline gates fit CI budgets | **Deterministic evaluator throughput** |
| Analyze/Replay are bit-stable on that path | **Reproducibility** (JCS hash stability) |
| Stochastic agents get honest verdicts | **H7** Wilson vectors (`PASS` / `FLAKY` / `FAIL`) |
| Curated adversarial catalog is classified correctly | **[Validation Suite]({% link benchmarks/validation.md %})** — pass/fail/flaky/infra/mutation/fixture/recovery/mode3 |

That is gust’s job: **test infrastructure for agents**. See [Metrics explained]({% link benchmarks/metrics.md %}) for definitions, and [Latest results]({% link benchmarks/results.md %}) for the published numbers (also [benchmark workflow runs](https://github.com/ShadyD45/gust/actions/workflows/benchmark.yml)).

## Latest snapshot

| Metric | Gate | What a pass means |
|--------|------|-------------------|
| Detection rate | ≥ 90% | Mutated golden-suite cases are caught |
| False positive rate | ≤ 5% | Unmutated golden runs stay green |
| Deterministic evaluator throughput | ≥ 1,000 evaluator calls/sec | Analyze/Replay path is CI-cheap |
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
