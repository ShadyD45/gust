---
title: What Gust is
nav_order: 2
parent: Architecture
---
# What Gust is

Gust is a **behavioral evaluation harness** for LLM agents. You describe a scenario (task + assertions + optional fixtures), Gust runs your real agent N times, grades the traces, and reports a Wilson reliability verdict — plus `gust-report.html`.

It is **not** an integration-test framework, a mocking platform, or a production observability product. You keep your agent framework, your DI/fakes, and your OTel backend. Gust owns sampling, contracts, scoring, and CI artifacts.

## What you use it for

| You want… | Gust gives you… |
|-----------|-----------------|
| “Did this change break tool choice / ordering / safety?” | Scenario assertions over live traces |
| “One PASS is not enough” | N-sample Wilson interval (`PASS` / `FAIL` / `FLAKY`) |
| Fast local/CI evals without rewriting the agent | Callable or command adapter (`exec` / `http` / `trigger`) |
| Controlled tool failures sometimes | Optional Gust fixtures (`world_control: gust`) |
| Remote QA with no Gust code in the agent | Your harness + W3C trace context + fetch-by-`{trace_id}` |

## How teams usually evaluate agents

Most agent eval stacks (dataset evals, LangSmith AgentEvals, OpenAI Agents testing, Microsoft Agent Framework testing) do **not** transparently intercept tools. They grade trajectories and leave world control to normal mocks, DI, fakes, or sandboxes.

Gust follows that practice:

1. **Default:** `world_control: existing` — use the mocks you already have  
2. **Optional:** `world_control: gust` — Gust fixture proxy for selected tools / failure injection (same-host)  
3. **Observation:** AgentRun on stdout/HTTP, OTLP push, or fetch from your existing backend  
4. **Gate:** deterministic assertions first; optional calibrated judges later

## Four pieces Gust coordinates

| Piece | What it does | Who owns it |
|-------|--------------|-------------|
| **Trigger** | Start one sample | `exec` / `http` / `trigger` + your callable or harness |
| **World control** | Real tools, your mocks, or Gust fixtures | You by default; Gust when you opt in |
| **Observation** | Normalize to `AgentRun` | Response body, OTLP, or `--trace-fetch-command` |
| **Evaluation** | Assertions, Wilson, policy, HTML/JSON | Gust |

## What Gust deliberately does not require

- Rewriting your agent to import Gust (a thin harness is enough)
- Deploying a Gust gateway or transparent HTTP/MCP proxy
- Pointing production traffic at Gust
- Replacing Langfuse / Tempo / Phoenix — Gust can fetch from them via a command you already use

Start in minutes:

```bash
# Adoption smoke (trigger IT harness → fetch trace → HTML report)
./demo/live-agent/run.sh --only integration,integration-unsafe

# Python hook against your own evals/
gust test evals/hello --runner exec --samples 5 -- python -m my_agent.gust_eval

# TypeScript
gust test evals/hello --runner exec --samples 5 -- node --import tsx my_agent/gust_eval.ts
```

Full walkthrough: [Getting started]({% link usage/getting-started.md %}). Live Mode 3 details: [Test your agent]({% link usage/test-your-agent.md %}). Component model: [How Gust works]({% link architecture/how-gust-works.md %}), [Mocking and fixtures]({% link architecture/mocking-and-fixtures.md %}), [Execution topologies]({% link architecture/execution-topologies.md %}). Embed in Go tests: [Go library]({% link usage/go-library.md %}).
