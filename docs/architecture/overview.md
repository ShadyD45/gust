---
title: Overview
nav_order: 1
parent: Architecture
---
# Overview

## Vision

gust is a **dataset-driven behavioral evaluation harness** for probabilistic agents — reproducible testing for autonomous software. Agent systems plan, call tools, retrieve context, retry, and execute multi-step workflows. Traditional unit tests assume deterministic execution; LLM agents do not. gust provides:

- Capture and analysis of execution traces (`AgentRun`)
- Optional controlled environments via fixtures and a same-host tool mock proxy
- Deterministic Replay for offline regression of recorded I/O
- Probabilistic Test-mode sampling with Wilson confidence intervals
- Policy-based CI gating (`PASS` / `FAIL` / `FLAKY`) and HTML/JSON artifacts
- Mutation testing as a trust metric for the evaluator suite

Product model: [What Gust is]({% link architecture/what-gust-is.md %}). Hands-on: [Getting started]({% link usage/getting-started.md %}). Component and topology details: [How Gust works]({% link architecture/how-gust-works.md %}), [Mocking and fixtures]({% link architecture/mocking-and-fixtures.md %}), [Execution topologies]({% link architecture/execution-topologies.md %}).

## Problem

An agent may pick the wrong tool, pass invalid arguments, loop, violate safety constraints, or succeed only accidentally. Final-answer checks miss trajectory failures. Worse, a single run that “passes” when the true success rate is ~85% is not a reliable regression signal.

## Design anchors

1. **A trace is not the test.** Extraction never auto-fills assertions from observed behavior — humans author the contract.
2. **Three modes stay distinct:** Analyze, Replay, Test.
3. **Reliability is statistical.** Test mode reports rates and intervals, not one-shot booleans.
4. **Deterministic stack first.** Evaluators, fixtures, Replay, and mutation testing run offline at zero model cost.

## Non-goals

- Replacing product analytics or online monitoring platforms
- Hosted dashboards or span explorers (use Phoenix/Langfuse; Gust stores eval artifacts)
- Transparent HTTP/MCP proxies or a mandatory Gust gateway
- Pointing production traffic at Gust

## Public surfaces

| Surface | For |
|---------|-----|
| `gust` CLI | Day-to-day analyze / replay / test / CI |
| Python / TypeScript SDKs | Record runs and Mode 3 hooks |
| [`pkg/api`](https://github.com/ShadyD45/gust/tree/main/pkg/api) | Shared types (`AgentRun`, assertions, results) |
| [`pkg/gust`](https://github.com/ShadyD45/gust/tree/main/pkg/gust) | Stable Go library API for embedding Analyze in your tests |

Application code should not import `gust/internal/...` — those packages are implementation details and may change without notice. See [Go library]({% link usage/go-library.md %}).
