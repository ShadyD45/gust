# Extension Points

## Registry

`internal/registry` holds thread-safe maps for:

- Evaluators
- Mutators
- Test runners

Built-ins register at init; CLI resolves components by name (`--runner synthetic`, mutator class lists, etc.). The CLI merges registry-registered evaluators and mutators with the built-in suites, so registering is sufficient to make a custom component available to `analyze`, `test`, and `mutate`.

Task-oriented walkthroughs live in [`docs/extending/`](../extending/); this page is the map.

## Adding an evaluator

1. Implement `ports.Evaluator` (`Name`, `Version`, `Evaluate`).
2. Register with the registry.
3. Assertion types resolve to the evaluator of the same name; extend `resolveEvaluatorName` only when remapping an existing type.
4. Prefer structured `evidence` maps for CI/debug output.

## Adding a mutator

1. Implement `ports.Mutator` with typed outcomes: `applied` | `skipped` | `error`.
2. Register by class name.
3. Mutation runner applies mutators then Analyze; skipped mutations must not inflate detection rate.

## Adding a test runner

1. Implement `ports.TestRunner` (`Name`, `Run(ctx, scenario, fixtureEndpoint)`).
2. Return a complete `AgentRun` trace.
3. Honor `fixtureEndpoint` / `AGENTEVAL_FIXTURE_ENDPOINT` so tools hit the mock proxy.
4. Register (e.g. `synthetic`, `ollama`).

## Tier-2 wire plugins

`internal/adapters/wire` hosts JSON-RPC 2.0 over stdio for cross-language plugins (Python/TypeScript evaluators). The supervisor manages process lifecycle, timeouts, and a scrubbed environment limited to the OS baseline needed to start an interpreter; plugin stderr is forwarded so failures are debuggable. Extend by implementing the wire methods expected by the client and wrapping them in a Go adapter that satisfies a port interface.

`sdk/python` provides `EvaluatorPlugin` + `serve()`, which implement the transport so a plugin author only writes `evaluate`.

## Ingestion adapters

`internal/adapters/ingest/otel` maps OTLP JSON exports using OpenInference semantic conventions onto `api.AgentRun` (`gust ingest otel`). Mapping is pure and deterministic; unmappable traces produce typed errors rather than partial runs. Add a format by implementing an equivalent mapper package and a subcommand under `gust ingest`.

## Fixture / store adapters

- In-memory fixture provider + HTTP mock proxy: `internal/adapters/fixtures`
- Filesystem JSON store: `internal/adapters/storage/filesystem` implementing `ScenarioStore`, `FixtureStore`, `RunStore`

Swap storage by implementing the store ports; core engines depend only on interfaces.
