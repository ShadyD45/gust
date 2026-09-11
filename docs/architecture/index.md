---
title: Architecture
nav_order: 5
has_children: true
has_toc: false
---
# Architecture

How gust is structured and why. Read in this order:

| Page | What it covers |
|------|----------------|
| [Overview]({% link architecture/overview.md %}) | Vision, problem framing, non-goals, public surfaces |
| [What Gust is]({% link architecture/what-gust-is.md %}) | Product model: evaluation harness, not an IT framework |
| [How Gust works]({% link architecture/how-gust-works.md %}) | End-to-end component model and data flow |
| [Mocking and fixtures]({% link architecture/mocking-and-fixtures.md %}) | World control, fixture proxy, fail-closed behavior |
| [Execution topologies]({% link architecture/execution-topologies.md %}) | Local, CI, Jenkins, QA, and OTel backend paths |
| [Execution modes]({% link architecture/execution-modes.md %}) | Analyze / Replay / Test, testing pyramid, and data flow |
| [Hexagonal design]({% link architecture/hexagonal-design.md %}) | Ports, adapters, package map |
| [Data model]({% link architecture/data-model.md %}) | Core types and content addressing |
| [Invariants]({% link architecture/invariants.md %}) | Hard contracts: evidence vs tests, modes, fixture isolation |
| [Statistics and policy]({% link architecture/statistics-and-policy.md %}) | Wilson intervals, verdicts, CI gating |
| [Extension points]({% link architecture/extension-points.md %}) | Registry, plugins, adding components |

Looking for task-oriented guides rather than design rationale? Start with [Getting started]({% link usage/getting-started.md %}), then [Usage]({% link usage/index.md %}), [Go library]({% link usage/go-library.md %}), [Tuning the gate]({% link usage/tuning.md %}), the [live-agent demo]({% link usage/live-agent-demo.md %}), and [Extending]({% link extending/index.md %}) for custom evaluators and plugins.
