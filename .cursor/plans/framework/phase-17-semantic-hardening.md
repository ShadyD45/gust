# Phase 17: Semantic Hardening & Adoption Front Door

## Status

**Shipped** with the review hardening pass (evaluator semantics, `gust init`, mutation UX, README front door).

## Objectives

1. Make evaluator semantics rigorous enough to earn the “testing infrastructure” label.
2. Lower time-to-first-value via `gust init` and a tighter README front door.
3. Make mutation testing’s trust signal (detected / escaped / score) prominent.

## P0 — Correctness

| Area | Change |
|------|--------|
| `tool_sequence` | `parameters.match`: `subsequence` (default) \| `exact` |
| `tool_arguments` | `parameters.occurrence`: `any` (default) \| `first` \| `last` \| 1-based index |
| `error_recovery` | Causal recovery: later successful same-op (or `recovery_tools`); optional `after_error_tool` |
| `max_latency` | `max(end) - min(start)` wall clock; `latency_source: wall_clock` only |
| `AgentRun.Validate` | Call existing `ValidateSpan` for type/status/times |

Primary code: `internal/adapters/evaluators/evaluators.go`, `pkg/api/types.go`.

## P1 — Adoption

- `gust init [dir]` scaffolds `gust.yaml` + `tests/{scenarios,fixtures,assertions,policy.yaml}`
- README: pitch → quick start → mutation score callout → deeper concepts below
- `gust mutate` terminal: Mutants / Detected / Escaped + Evaluator mutation score

## Out of scope

- Real LLM agent end-to-end demo → **Phase 18**
- Stateful Environment redesign, UI, Phases 13–16 feature work

## Exit criteria

- Unit tests for subsequence vs exact, occurrence modes, unrelated-success recovery false positive, unordered latency spans, invalid span type via `Validate()`
- `go test` green for touched packages
- `gust init` and mutation score output documented in README / modes cookbook
