# Hexagonal Design

gust uses ports & adapters so domain logic stays free of I/O frameworks and LLM SDKs.

```text
                 +---------------------------+
                 |        cmd/gust           |
                 |      internal/cli         |
                 +-------------+-------------+
                               |
                 +-------------v-------------+
                 |      internal/core        |
                 | analyze replay test       |
                 | mutate policy scenario    |
                 | stats dataset             |
                 +------+-------------+------+
                        |             |
              +---------v--+     +----v----------+
              | ports      |     | domain/evidence|
              +---------+--+     +---------------+
                        |
        +---------------+----------------+
        | adapters                       |
        | evaluators mutators fixtures   |
        | testrunner wire storage        |
        +--------------------------------+
```

## Layers

| Layer | Packages | Responsibility |
|-------|----------|----------------|
| **Public API** | `pkg/api`, `pkg/jcs` | Domain types, validation, canonical hashing |
| **Ports** | `internal/ports` | `Evaluator`, `Mutator`, `FixtureProvider`, `TestRunner`, stores |
| **Core** | `internal/core/*` | Use cases: modes, stats, policy, extraction |
| **Adapters** | `internal/adapters/*` | Built-in implementations and I/O |
| **Entry** | `cmd/gust`, `internal/cli` | Cobra CLI, formatters, exit codes |

## Package map (MVP)

```text
gust/
├── cmd/gust/
├── pkg/api/
├── pkg/jcs/
├── internal/
│   ├── ports/
│   ├── registry/
│   ├── domain/evidence/
│   ├── core/
│   │   ├── analyze/
│   │   ├── replay/
│   │   ├── test/
│   │   ├── stats/
│   │   ├── mutate/
│   │   ├── policy/
│   │   ├── scenario/
│   │   └── dataset/
│   ├── adapters/
│   │   ├── evaluators/
│   │   ├── mutators/
│   │   ├── fixtures/
│   │   ├── testrunner/
│   │   ├── wire/
│   │   └── storage/filesystem/
│   └── cli/
└── spec/schemas/
```

## Concurrency

Mode 3 sampling and mutation batches use bounded worker pools (`sync.WaitGroup` + semaphore channel). All blocking APIs take `context.Context` for cancellation.
