---
title: Hexagonal design
nav_order: 3
parent: Architecture
has_mermaid: true
---
# Hexagonal Design

gust uses ports & adapters so domain logic stays free of I/O frameworks and LLM SDKs.

```mermaid
flowchart TB
  accTitle: gust hexagonal architecture
  accDescr: CLI entry calls core use cases, which depend on ports and domain evidence. Adapters implement the ports.

  entry["cmd/gust · internal/cli"]
  core["internal/core<br/>analyze · replay · test<br/>mutate · policy · scenario<br/>stats · dataset"]
  ports[internal/ports]
  evidence[domain/evidence]
  adapters["adapters<br/>evaluators · mutators · fixtures<br/>testrunner · wire · storage"]

  entry --> core
  core --> ports
  core --> evidence
  ports --> adapters
```

## Layers

| Layer | Packages | Responsibility |
|-------|----------|----------------|
| **Public API** | `pkg/api`, `pkg/jcs` | Domain types, validation, canonical hashing |
| **Ports** | `internal/ports` | `Evaluator`, `Mutator`, `FixtureProvider`, `TestRunner`, stores |
| **Core** | `internal/core/*` | Use cases: modes, stats, policy, extraction |
| **Adapters** | `internal/adapters/*` | Built-in implementations and I/O |
| **Entry** | `cmd/gust`, `cmd/gust-aee`, `internal/cli` | End-user Cobra CLI; internal self-benchmark CLI |

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
│   │   ├── ingest/otel · langfuse
│   │   ├── testrunner/
│   │   ├── wire/
│   │   └── storage/filesystem/
│   └── cli/
└── spec/schemas/
```

## Concurrency

Mode 3 sampling and mutation batches use bounded worker pools (`sync.WaitGroup` + semaphore channel). All blocking APIs take `context.Context` for cancellation.

