# Phase 18: Real-Agent End-to-End Example

## Status

**Deferred / planned stub.** Not part of the Phase 17 hardening implementation.

## Objective

Give adopters one realistic end-to-end path with:

```text
real LLM
  → real tool calls
  → intentional failure
  → Gust detects it
```

The synthetic runner remains valuable for statistics machinery; this phase proves Gust on a live agent stack.

## Starting point

Build on the existing LangChain demo scaffold:

- [`demo/frameworks/langchain/`](../../../demo/frameworks/langchain/)

## Deliverables (when scheduled)

1. Documented script that runs a real (or locally mocked LLM) agent through Mode 3 with fixtures.
2. A deliberate buggy path where assertions / mutation / analyze surface the failure clearly.
3. Link from README Quick start as an optional “real agent” track (after synthetic demo).

## Dependencies

- Phase 17 semantic hardening complete (evaluator trust).
- Prefer local Ollama or recorded fixtures so CI stays offline-capable where possible.

## Non-goals

- Hosted UI / observability dashboard
- Replacing the synthetic killer demo
