# gust MVP: Implementation Architecture & Plan Index

## 1. Executive Summary

This directory contains the step-by-step engineering implementation plan for the **gust MVP** (Phases 1 through 9).

The codebase is built in **Go 1.23+** as a single, statically compiled binary with zero external runtime dependencies. It is architected for maximum extensibility, thread-safe concurrent performance, modular plugin adapters, and clean separation of concerns.

---

## 2. Architectural Principles & Go Idioms

### 2.1 Clean Architecture / Ports & Adapters (Hexagonal)
The codebase strictly adheres to Hexagonal Architecture:
- **Domain Layer (`pkg/api`, `internal/domain`):** Pure entities (`AgentRun`, `Span`, `TestScenario`, `Fixture`, `Assertion`, `Policy`). Zero external dependencies.
- **Port Interfaces (`internal/ports`):** Interfaces defining the contracts required by domain logic (`Evaluator`, `Mutator`, `FixtureProvider`, `TestRunner`, `PolicyEngine`).
- **Core Use Cases (`internal/core/`):** Implementation of the three execution modes:
  - `internal/core/analyze`: Mode 1 (static analysis of captured traces).
  - `internal/core/replay`: Mode 2 (deterministic replay against recorded fixtures).
  - `internal/core/test`: Mode 3 (probabilistic testing with live agents and LLMs).
  - `internal/core/mutate`: Mutation testing engine.
  - `internal/core/policy`: Policy evaluation and regression comparison.
  - `internal/core/scenario`: Assisted scenario extraction.
- **Adapters (`internal/adapters/`):** Pluggable implementations:
  - `internal/adapters/evaluators`: The 10 built-in deterministic evaluators.
  - `internal/adapters/mutators`: The 9 built-in mutation classes.
  - `internal/adapters/testrunner`: Local Ollama, synthetic, and HTTP test runners.
  - `internal/adapters/fixtures`: Content-addressed file store and mock tool proxy server.
  - `internal/adapters/wire`: JSON-RPC 2.0 stdio cross-language plugin host.
- **Entry Points (`cmd/gust`, `internal/cli`):** Cobra-based CLI commands.

### 2.2 Go Best Practices Enforced
1. **Interface Segregation:** Interfaces are minimal, focused, and declared where they are consumed.
2. **Context Propagation:** All blocking, I/O, or long-running methods accept `context.Context` as their first parameter for clean timeout and cancellation handling.
3. **Value Semantics & Immutability:** Domain objects passed across evaluation pipelines use value semantics or immutable references to prevent concurrent mutation bugs.
4. **Structured Error Handling:** Domain errors use typed sentinel errors (`ErrFixtureNotFound`, `ErrSchemaViolation`, `ErrInsufficientSamples`) wrapped with `fmt.Errorf("%w", err)`.
5. **Worker Pool Concurrency:** Mode 3 parallel sampling and mutation batching use bounded worker pools (`golang.org/x/sync/errgroup` with concurrency limits) to prevent system starvation or rate-limit saturation.
6. **Zero Reflection in Hot Paths:** Trajectory iteration and step evaluation minimize allocations and reflection to easily exceed the target of $\ge 1,000$ cases/sec per core.

---

## 3. Package Directory Layout

```text
gust/
├── cmd/
│   └── gust/               # Main CLI entrypoint (main.go)
├── pkg/
│   ├── api/                     # Public domain types, constants, schemas
│   └── jcs/                     # RFC 8785 JSON Canonicalization Scheme
├── internal/
│   ├── domain/                  # Domain entities & validations
│   ├── ports/                   # Go interface definitions (Tier 1)
│   ├── core/
│   │   ├── analyze/             # Mode 1: Trace analysis engine
│   │   ├── replay/              # Mode 2: Deterministic replay engine
│   │   ├── test/                # Mode 3: Probabilistic testing engine
│   │   ├── stats/               # Wilson score & statistical calculations
│   │   ├── mutate/              # Mutation testing engine
│   │   ├── policy/              # Policy evaluation engine & regression comparator
│   │   └── scenario/            # Assisted extraction workflow
│   ├── adapters/
│   │   ├── evaluators/          # 10 built-in deterministic evaluators
│   │   ├── mutators/            # 9 built-in mutation classes
│   │   ├── fixtures/            # Fixture store, stateful queues, mock proxy
│   │   ├── testrunner/          # Ollama/local LLM & mock test runners
│   │   └── wire/                # Tier 2 JSON-RPC stdio plugin supervisor
│   └── cli/                     # Cobra CLI commands, flags, formatters
├── spec/
│   └── schemas/                 # JSON Schemas (Draft 2020-12)
├── tests/
│   ├── testdata/                # Golden traces, synthetic agents, fixtures
│   ├── unit/                    # Unit tests
│   ├── integration/             # Integration tests
│   └── benchmark/               # Throughput & mutation benchmarks
├── docs/
│   └── plans/
│       ├── roadmap.md           # Master Phased Roadmap (Phases 0 - 16)
│       └── mvp/                 # MVP Implementation Plans (Phases 1 - 9)
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

---

## 4. MVP Implementation Phase Index

Click on any phase below for its full engineering specification, interface contracts, error models, and verification steps:

| Phase | Plan Document | Scope & Deliverables |
|---|---|---|
| **Phase 1** | [`phase-01-core-types-and-schema.md`](file:///d:/Projects/gust/docs/plans/mvp/phase-01-core-types-and-schema.md) | Domain types, JSON Schema validation (Draft 2020-12), and RFC 8785 Canonical JCS hashing. |
| **Phase 2** | [`phase-02-interfaces-and-wire-protocol.md`](file:///d:/Projects/gust/docs/plans/mvp/phase-02-interfaces-and-wire-protocol.md) | Tier 1 Go contracts, Tier 2 JSON-RPC 2.0 stdio wire protocol, plugin supervisor (env scrub + timeouts; not OS sandboxing). |
| **Phase 3** | [`phase-03-deterministic-evaluators.md`](file:///d:/Projects/gust/docs/plans/mvp/phase-03-deterministic-evaluators.md) | The 10 MVP deterministic evaluators, structured evidence generation, $\ge 1,000$ cases/sec throughput. |
| **Phase 4** | [`phase-04-fixture-store-replay-analyze.md`](file:///d:/Projects/gust/docs/plans/mvp/phase-04-fixture-store-replay-analyze.md) | Content-addressed Fixture Store, stateful sequential queues, Tool Mock Proxy Server, Mode 1 & 2. |
| **Phase 5** | [`phase-05-mutation-testing-engine.md`](file:///d:/Projects/gust/docs/plans/mvp/phase-05-mutation-testing-engine.md) | 9 mutation classes, Replay-mode mutation runner, Detection Rate ($\ge 90\%$) and FPR ($\le 5\%$). |
| **Phase 6** | [`phase-06-test-mode-and-reliability.md`](file:///d:/Projects/gust/docs/plans/mvp/phase-06-test-mode-and-reliability.md) | Mode 3 Test Runner, parallel sampling, Ollama/local LLM, Wilson score statistical engine, `FLAKY` classification. |
| **Phase 7** | [`phase-07-policy-engine.md`](file:///d:/Projects/gust/docs/plans/mvp/phase-07-policy-engine.md) | Policy Engine, hard vs soft constraints, `on_flaky` actions (`warn`/`fail`/`ignore`), regression comparator. |
| **Phase 8** | [`phase-08-scenario-extraction.md`](file:///d:/Projects/gust/docs/plans/mvp/phase-08-scenario-extraction.md) | `gust scenario from-run` extraction tooling, structural assertion protection against bug enshrinement. |
| **Phase 9** | [`phase-09-cli-ci-and-demo.md`](file:///d:/Projects/gust/docs/plans/mvp/phase-09-cli-ci-and-demo.md) | Cobra CLI commands, CI exit codes, JSON/table formatters, demo end-to-end verification. |
