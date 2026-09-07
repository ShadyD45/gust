---
title: Overview
nav_order: 1
parent: Architecture
---
# Overview

## Vision

gust is **test infrastructure for autonomous software** — reproducible testing for probabilistic agents. Agent systems plan, call tools, retrieve context, retry, and execute multi-step workflows. Traditional unit tests assume deterministic execution; LLM agents do not. gust provides:

- Capture and analysis of execution traces (`AgentRun`)
- Controlled environments via fixtures and a tool mock proxy
- Deterministic Replay for offline regression of recorded I/O
- Probabilistic Test-mode sampling with Wilson confidence intervals
- Policy-based CI gating (`PASS` / `FAIL` / `FLAKY`)
- Mutation testing as a trust metric for the evaluator suite

## Problem

An agent may pick the wrong tool, pass invalid arguments, loop, violate safety constraints, or succeed only accidentally. Final-answer checks miss trajectory failures. Worse, a single run that “passes” when the true success rate is ~85% is not a reliable regression signal.

## Design anchors

1. **A trace is not the test.** Extraction never auto-fills assertions from observed behavior (Hypothesis H8).
2. **Three modes stay distinct:** Analyze, Replay, Test.
3. **Reliability is statistical.** Test mode reports rates and intervals, not one-shot booleans.
4. **Deterministic stack first.** Evaluators, fixtures, Replay, and mutation testing run offline at zero model cost.

## Non-goals

- Replacing product analytics or online monitoring platforms
- Claiming Gust is “proven correct” — the self-benchmark continuously validates evaluation machinery against adversarial and mutation-based suites
- Runtime Draft 2020-12 JSON Schema library validation (schemas are contracts; Go uses hand validation)
- Hosted dashboards or span explorers (use Phoenix/Langfuse; Gust stores eval artifacts)
- Session-based remote fixture backends (`FixtureProvider.NewSession`) — per-sample proxies isolate Mode 3 today; a session protocol is future work for SQLite/Redis/container fixtures

## MVP boundary

| Phases | Scope |
|--------|--------|
| **1–5** | Types, ports/wire, evaluators, fixtures, Analyze/Replay, mutation |
| **6–9** | Mode 3 + Wilson, policy/regression, scenario extraction, CLI/CI demo |
| **10–12, 17, 20** | OTel ingest, SDKs, optional LLM judge + `judge_panel`, AEE, semantic hardening, fixture isolation / classified retry |
| **18** | Real-agent E2E — [live-agent demo]({% link usage/live-agent-demo.md %}) (scripted fallback + Ollama) |

## Implementation principles

- Hexagonal architecture (ports & adapters)
- `context.Context` on all I/O and long-running work
- Minimal dependencies: Cobra + yaml.v3 at the edges; core engines stdlib-only
- Content addressing via RFC 8785 JCS + SHA-256

