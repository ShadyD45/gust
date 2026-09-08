---
title: Extension points
nav_order: 6
parent: Architecture
---
# Extension points

How Gust stays extensible without requiring you to fork the binary.

## For application teams

| Need | Extension | Guide |
|------|-----------|-------|
| Domain assertion in Go tests | `gust.Evaluator` via `pkg/gust` | [Go library]({% link usage/go-library.md %}), [Custom evaluator]({% link extending/custom-evaluator-go.md %}) |
| Domain assertion in Python/TS | `--plugin` wire plugin | [Wire plugins]({% link extending/wire-plugin-python.md %}) |
| Live Mode 3 against your agent | `--runner exec\|http\|trigger` | [Test your agent]({% link usage/test-your-agent.md %}) |
| Optional LLM grading | `llm_judge` / `judge_panel` + calibration | [LLM judge]({% link usage/llm-judge.md %}) |
| OTel / Langfuse traces | `gust ingest otel` / `langfuse` | [OTel ingestion]({% link usage/otel-ingest.md %}) |

Application code should import only:

- `gust/pkg/api` — types
- `gust/pkg/gust` — Analyze library API
- Language SDKs under `sdk/`

Do not import `gust/internal/...`.

## Wire plugins (cross-language)

Gust starts a subprocess that speaks JSON-RPC 2.0 over stdin/stdout. Configuration: `gust.yaml` `wire.handshake_timeout` (default 5s), `wire.evaluate_timeout` (default 60s), with a 16 MiB line-size cap. Plugin stderr is forwarded for debugging.

**Trust boundary today:** environment scrubbing + timeouts + line bounds. Full CPU/memory/network sandboxing is not claimed yet — treat untrusted community plugins accordingly.

`sdk/python` provides `EvaluatorPlugin` + `serve()` so authors only implement `evaluate`.

## Optional LLM judge

`llm_judge` and `judge_panel` are opt-in. Major providers use official Python SDKs via `--plugin` / `--judge-plugin`. Policy flag `allow_llm_judge` defaults to false. See [LLM judge]({% link usage/llm-judge.md %}) and [Judge calibration]({% link usage/judge-calibration.md %}).

## Ingestion

- **OTLP:** `gust ingest otel serve` or the in-process listener inside `gust test` (HTTP :4318 + gRPC :4317, plus `POST /v1/runs`). CI/laptop infra — not a production sidecar.
- **Offline:** `--file`, stdin, or `--url`.
- **Langfuse:** pull one trace or session.

## Fixtures and stores

Gust fixtures are an optional world-control mode (`world_control: gust`). Scenarios and runs can be stored as files; content-addressed bundles use `gust dataset bundle` / `verify`. See [Mocking and fixtures]({% link architecture/mocking-and-fixtures.md %}).
