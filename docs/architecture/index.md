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
| [Overview]({% link architecture/overview.md %}) | Vision, problem framing, non-goals, MVP boundary |
| [Execution modes]({% link architecture/execution-modes.md %}) | Analyze / Replay / Test, testing pyramid, and data flow |
| [Hexagonal design]({% link architecture/hexagonal-design.md %}) | Ports, adapters, package map |
| [Data model]({% link architecture/data-model.md %}) | Core types and content addressing |
| [Statistics and policy]({% link architecture/statistics-and-policy.md %}) | Wilson intervals, verdicts, CI gating |
| [Extension points]({% link architecture/extension-points.md %}) | Registry, plugins, adding components |

Looking for task-oriented guides rather than design rationale? See [Usage]({% link usage/index.md %}) to run gust against your own agent, the [live-agent demo]({% link usage/live-agent-demo.md %}) for a recorded Mode 3 run, and [Extending]({% link extending/index.md %}) to add evaluators, runners, or cross-language plugins.
