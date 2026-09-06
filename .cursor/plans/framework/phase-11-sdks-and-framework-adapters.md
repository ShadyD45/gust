# Phase 11: SDKs and Framework Adapters

## Status

**Shipped.** Python SDK (`RunRecorder`, `FixtureClient`, `EvaluatorPlugin`, `run_sample` / `serve_sample`), TypeScript SDK (`sdk/typescript`), LangChain adapter (`gust-sdk[langchain]`), and compatibility matrix (`docs/usage/compatibility.md`).

**Follow-up — enable artifact publish:** Re-enable [`.github/workflows/publish-npm.yml`](../../../.github/workflows/publish-npm.yml) and [`.github/workflows/publish-pypi.yml`](../../../.github/workflows/publish-pypi.yml) after adding `NPM_TOKEN` and `PYPI_API_TOKEN` as repository secrets. Both workflows are currently `if: false` so they never run on tags or `workflow_dispatch`. Remove that guard, then publish `sdk/typescript` (npm) and `sdk/python` (PyPI) on `v*` tags.

## Objectives

1. Publish lightweight **Python** and **TypeScript** client SDKs that emit `AgentRun` / call the wire protocol or HTTP fixture proxy.
2. Provide adapters for popular agent stacks: **LangChain**, **LlamaIndex**, **AutoGen**, **CrewAI** (priority order by adoption).
3. Keep the Go binary as the evaluation authority; SDKs are capture + invoke clients.

## Scope

| In | Out |
|----|-----|
| `sdk/python`, `sdk/typescript` packages | Rewriting core engines in Python/TS |
| Decorators / callbacks that record spans | Hosted SaaS control plane |
| Docs: install, record, `gust test`/`analyze` | Full MCP host rewrite |

## Architecture

```text
Framework agent ──► SDK tracer ──► AgentRun JSON / OTel
                                      │
                                      ▼
                              gust CLI / library (Go)
```

Tier-2 JSON-RPC wire remains available for custom evaluators written in Python/TS.

## Deliverables

1. PyPI / npm packages with minimal deps.
2. One reference adapter (LangChain) with demo under `demo/frameworks/`.
3. Compatibility matrix in docs (framework version × gust schema version).

## Verification

- SDK round-trip: record → write run.json → `gust analyze` passes golden assertions.
- Fixture proxy env (`AGENTEVAL_FIXTURE_ENDPOINT`) works from Python and Node agents.
- CI builds SDK packages on every main push (lint + unit).

## Exit criteria

A non-Go developer can instrument a LangChain agent and run Mode 1 + Mode 3 against gust without reading Go source.
