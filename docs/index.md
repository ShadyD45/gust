---
title: Home
nav_order: 1
description: Test what happens when your agent meets a world that doesn’t behave.
permalink: /
layout: default
---

<div class="home-hero" markdown="0">
  <img src="{{ '/assets/images/gust-logo.png' | relative_url }}" alt="gust — Test what happens when your agent meets a world that doesn’t behave." width="420" height="183">
  <p class="home-kicker">Test infrastructure for autonomous software</p>
</div>

gust is the role that JUnit, a mocking framework, and a CI regression gate play for ordinary backends — adapted for LLM-driven agents that do not give the same answer twice.

It is **not** a final-answer LLM evaluation library. It measures agent *behavior* (tool choice, arguments, sequences, safety constraints, latency) and treats correctness as a **statistical pass rate** with confidence intervals.

{: .important }
A **trace is not a test**. A captured `AgentRun` is evidence of what happened once. A `TestScenario` (task + fixtures + independent assertions) is the test artifact. gust never auto-fills assertions from observed behavior.

## Start here

| Goal | Guide |
|------|--------|
| Wire gust into an agent you already run | [Integrate your app]({% link usage/integrate-your-app.md %}) |
| Learn Analyze / Replay / Test / Compare | [Modes cookbook]({% link usage/modes-cookbook.md %}) |
| See self-benchmark proof (AEE) | [Benchmarks]({% link benchmarks/index.md %}) |
| Convert OpenTelemetry traces | [OTel ingestion]({% link usage/otel-ingest.md %}) |
| Put a gate in GitHub Actions (gust on CI; agent in the job or QA) | [CI integration]({% link usage/ci-github-actions.md %}) |
| Add a custom evaluator or runner | [Extending gust]({% link extending/index.md %}) |
| Understand the design | [Architecture]({% link architecture/index.md %}) |

## Three execution modes

| Mode | What it does | Network / LLM |
|------|----------------|---------------|
| **Analyze** | Evaluate assertions against a captured `AgentRun` | Offline |
| **Replay** | Re-drive recorded tool I/O via fixtures | Offline |
| **Test** | Run the real agent against mocked tools, *N* times | Dev/QA or in-CI agent — not production |

```bash
go build -o gust ./cmd/gust
./gust analyze testdata/runs/golden_cancel.json --policy testdata/policy.yaml
./demo/run.sh   # or ./demo/run.ps1 on Windows
```

## Source & contributing

- Repository: [ShadyD45/gust](https://github.com/ShadyD45/gust)
- [Contributor guide](https://github.com/ShadyD45/gust/blob/main/CONTRIBUTING.md)
- License: Apache-2.0
