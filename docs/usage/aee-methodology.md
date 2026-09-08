---
title: AEE methodology
nav_order: 8
parent: Usage
---
# Agent Evaluation Effectiveness (AEE)

AEE scores **gust's evaluation suite**, not an agent under test. It answers: can this suite catch real failures, avoid false alarms, stay fast, and reproduce?

```text
AEE ≈ f(detection_rate, false_positive_rate, reproducibility, eval_latency)
```

This is a **self-benchmark** only. It measures gust's own suite: replay, mutation detection, Wilson CI gates, and reproducibility.

For definitions of each metric and the product narrative, start with the site section: **[Benchmarks]({% link benchmarks/index.md %})**.

## How to reproduce

```bash
go build -o gust-aee ./cmd/gust-aee
./gust-aee report
./gust-aee report --json --out aee.json
./gust-aee report --doc benchmarks/RESULTS.md --site-doc docs/benchmarks/results.md
```

Maintainer/CI only — end users install **`gust`**, not `gust-aee`.

Or in tests:

```bash
go test ./... -count=1
```

## Gust Validation Suite

Complementary adversarial catalog (`gust-aee validate`) — crafted cases answering “should this pass / fail / be flaky / be infra / mutation / fixture / recovery / Mode 3 isolation?”. Gate requires every case in **this catalog** to match its authored expectation. That is regression evidence for Gust’s evaluation machinery, not a proof that Gust correctly evaluates arbitrary real-world agents.

```bash
./gust-aee validate
./gust-aee validate --out benchmarks/fixtures/gust_validation.json \
  --doc benchmarks/VALIDATION.md \
  --site-doc docs/benchmarks/validation.md
```

See [Validation Suite]({% link benchmarks/validation.md %}).

## Inputs

| Input | Measurement | Gate |
|-------|-------------|------|
| Detection rate | Mutation engine on the golden suite | >= 90% |
| False positive rate | Same golden suite, unmutated | <= 5% |
| Deterministic evaluator throughput | In-process evaluator calls only (no `llm_judge`, not `gust test`) | >= 1,000 evaluator calls/sec |
| Reproducibility | Canonical hash of analyze results (timing excluded) over 20 trials | 100% identical |
| Reliability engine | Wilson vectors 100/100→PASS, 20/20→FLAKY, 17/20→FAIL | all correct |

Composite `aee_score` is a weighted blend of the gates above. The boolean `passed` field is what CI should gate on.

Latest numbers: [Latest results]({% link benchmarks/results.md %}) (also [`benchmarks/RESULTS.md`](../../benchmarks/RESULTS.md) in the repo) · [benchmark workflow runs](https://github.com/ShadyD45/gust/actions/workflows/benchmark.yml).

## What is excluded

- Mode 3 agent pass rates (property of the *agent*, not the suite)
- LLM judge latency / agreement (optional soft signal; calibrate separately)
