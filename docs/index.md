---
title: Home
nav_order: 1
description: Test what happens when your agent meets a world that doesn’t behave.
permalink: /
layout: default
---

<div class="home-hero" markdown="0">
  <img src="{{ '/assets/images/gust-logo.png' | relative_url }}" alt="gust — Test what happens when your agent meets a world that doesn’t behave." width="420" height="183">
  <p class="home-kicker">Reproducible testing for probabilistic agents</p>
</div>

gust is test infrastructure for LLM-driven agents that do not give the same answer twice — offline analysis, fixture replay, and statistical CI gates over agent behavior.

It is **not** a final-answer LLM evaluation library. It measures agent *behavior* (tool choice, arguments, sequences, safety constraints, latency) and treats correctness as a **statistical pass rate** with confidence intervals.

{: .important }
A **trace is not a test**. A captured `AgentRun` is evidence of what happened once. A `TestScenario` (task + fixtures + independent assertions) is the test artifact. gust never auto-fills assertions from observed behavior.

## Start here

| Goal | Guide |
|------|--------|
| First useful gate in ~15 minutes | [Getting started]({% link usage/getting-started.md %}) |
| Prove live-eval adoption (trigger→fetch→HTML) | [`demo/live-agent` integration](https://github.com/ShadyD45/gust/tree/main/demo/live-agent/integration) |
| What Gust is (and is not) | [What Gust is]({% link architecture/what-gust-is.md %}) |
| Wire Gust into an agent you already run | [Integrate your app]({% link usage/integrate-your-app.md %}) |
| Embed Analyze in Go tests | [Go library]({% link usage/go-library.md %}) |
| Live N-sample eval against your agent | [Test your agent]({% link usage/test-your-agent.md %}) |
| Run the live-agent Mode 3 demo (N=20 Wilson) | [Live-agent demo]({% link usage/live-agent-demo.md %}) |
| Learn Analyze / Replay / Test / Compare | [Modes cookbook]({% link usage/modes-cookbook.md %}) |
| Tune policy, criticality, and CI knobs | [Tuning the gate]({% link usage/tuning.md %}) |
| Copy YAML for retail, RAG, SRE, PR review, clinic, SQL | [Scenario examples]({% link usage/examples.md %}) |
| See self-benchmark (AEE + Validation Suite) | [Benchmarks]({% link benchmarks/index.md %}) |
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
./demo/run.sh                 # synthetic MVP CLI demo
./demo/live-agent/run.sh      # Mode 3: exec + trigger/IT ingest + HTML (N=20)
./demo/live-eval/run.sh       # adoption subset (integration,integration-unsafe)
```

## Source & contributing

- Repository: [ShadyD45/gust](https://github.com/ShadyD45/gust)
- [Contributor guide](https://github.com/ShadyD45/gust/blob/main/CONTRIBUTING.md)
- License: Apache-2.0
