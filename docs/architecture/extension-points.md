---
title: Extension points
nav_order: 6
parent: Architecture
---
# Extension Points

## Registry

`internal/registry` holds thread-safe maps for:

- Evaluators
- Mutators
- Test runners

Built-ins register at init; CLI resolves components by name (`--runner synthetic`, mutator class lists, etc.). The CLI merges registry-registered evaluators and mutators with the built-in suites, so registering is sufficient to make a custom component available to `analyze`, `test`, and `mutate`.

Task-oriented walkthroughs live in [Extending]({% link extending/index.md %}); this page is the map.

## Adding an evaluator

1. Implement `ports.Evaluator` (`Name`, `Version`, `Evaluate`).
2. Register with the registry.
3. Assertion types resolve to the evaluator of the same name; extend `resolveEvaluatorName` only when remapping an existing type.
4. Prefer structured `evidence` maps for CI/debug output.

## Optional LLM judge

`ports.JudgeProvider` backs the built-in `llm_judge` evaluator. `judge_panel` aggregates several registered judge evaluators. Major providers use **official Python SDKs** (`gust_sdk.judge`) loaded with `--plugin` / `--judge-plugin`. Go ships only `mock` and a thin `generic` OpenAI-compatible HTTP adapter. See [LLM judge]({% link usage/llm-judge.md %}). Policy flag `allow_llm_judge` defaults to false.

## Adding a mutator

1. Implement `ports.Mutator` with typed outcomes: `applied` | `skipped` | `error`.
2. Register by class name.
3. Mutation runner applies mutators then Analyze; skipped mutations must not inflate detection rate.

## Adding a test runner

End users invoke their agent with `gust test --runner http|exec` — see [Test your agent]({% link usage/test-your-agent.md %}). Built-ins: `synthetic`, `ollama`, `http`, `exec`. The [live-agent demo]({% link usage/live-agent-demo.md %}) uses `exec` plus an optional Ollama-driven Python hook.

Go embedders:

1. Implement `ports.TestRunner` (`Name`, `Run(ctx, scenario, fixtureEndpoint)`).
2. Return a complete `AgentRun` trace.
3. Honor `fixtureEndpoint` / `AGENTEVAL_FIXTURE_ENDPOINT` so tools hit the mock proxy.
4. Register (e.g. a domain-specific runner).

## Tier-2 wire plugins

`internal/adapters/wire` hosts JSON-RPC 2.0 over stdio for cross-language plugins (Python/TypeScript evaluators). The supervisor manages process lifecycle with a **scrubbed environment**, a **bounded handshake** (default 5s, `gust.yaml` `wire.handshake_timeout`), a **default per-call evaluate timeout** (default 60s when the caller has no deadline, `gust.yaml` `wire.evaluate_timeout`), and a **16 MiB line-size cap** on protocol messages. Plugin stderr is forwarded so failures are debuggable.

**Trust boundary today (honest):** env scrubbing + timeouts + line bounds. CPU/memory/network sandboxing (cgroups, Job Objects, network namespaces) is **not yet implemented** — treat that as Phase 8.5 before advertising untrusted community plugins as fully sandboxed.

Extend by implementing the wire methods expected by the client and wrapping them in a Go adapter that satisfies a port interface.

`sdk/python` provides `EvaluatorPlugin` + `serve()`, which implement the transport so a plugin author only writes `evaluate`.

## Ingestion adapters

`internal/adapters/ingest/otel` maps OTLP JSON (and protobuf/gRPC) using OpenInference conventions onto `api.AgentRun`. Live ingest is a **session** — `gust ingest otel serve` or the in-process listener inside `gust test` (HTTP :4318 + gRPC :4317, plus `POST /v1/runs`). It is CI/laptop infra, not a production sidecar. Offline ingest is `--file`, stdin, or `--url`. `internal/adapters/ingest/langfuse` pulls one Langfuse trace or session. Mapping is pure and deterministic; unmappable traces produce typed errors rather than partial runs. Add a format by implementing an equivalent mapper package and a subcommand under `gust ingest`.

## Fixture / store adapters

- In-memory fixture provider + HTTP mock proxy: `internal/adapters/fixtures`
- Filesystem JSON store: `internal/adapters/storage/filesystem` implementing `ScenarioStore`, `FixtureStore`, `RunStore`. This is a **mutable local cache** (same ID overwrites). Content-addressed immutability is `gust dataset bundle` / `verify`.

`ports.RunStore` is the hook for a later eval-history database (user-owned SQLite/Postgres). It stores `AgentRun` documents and will store verdicts — not a span explorer. `gust ingest otel serve --output-dir` is the file-shaped stand-in today. Do not add a dashboard here; live ingest already calls `OnRun`, which a store adapter can implement as `SaveRun`.

Swap storage by implementing the store ports; core engines depend only on interfaces.

