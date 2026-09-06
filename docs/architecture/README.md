# gust Architecture

This directory describes how gust is structured and why. Read in this order:

1. **[overview.md](overview.md)** — Vision, problem framing, non-goals, MVP boundary
2. **[execution-modes.md](execution-modes.md)** — Analyze / Replay / Test and data flow
3. **[hexagonal-design.md](hexagonal-design.md)** — Ports, adapters, package map
4. **[data-model.md](data-model.md)** — Core types and content addressing
5. **[statistics-and-policy.md](statistics-and-policy.md)** — Wilson intervals, verdicts, CI gating
6. **[extension-points.md](extension-points.md)** — Registry, plugins, adding components

Canonical product requirements live in [`gust_Specification_v0.5.md`](../../gust_Specification_v0.5.md). Implementation phase plans live in [`docs/plans/mvp/`](../plans/mvp/).
