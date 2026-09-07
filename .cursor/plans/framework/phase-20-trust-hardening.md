# Phase 20: Trust Hardening

## Status

**Shipped.** Implements the architect review “Gust Trust Phase” except real-agent E2E, which remains [Phase 18](phase-18-real-agent-e2e.md).

## Objective

Harden Mode 3 contracts so concurrent samples cannot share fixture state, make retry and policy fields mean what the schema says, expose tunables in `gust.yaml` / policy / CLI, and keep documentation claims aligned with what AEE and the Validation Suite actually measure.

## In scope (done)

1. **Per-sample fixture isolation** — clone clonable providers and start an ephemeral mock-tool proxy per sample (`SamplingConfig.ProxyFactory`). Ordered fixtures stay parallel when the provider can clone.
2. **Mode 3 validation category** (synthetic HTTP fixture probe, **no live LLM**) — concurrent ordered/exact fixtures, isolation, exhaustion, retry-reset.
3. **RetryPolicy** — `none` | `transient` | `all`; defaults `max_attempts=2`, `backoff_ms=50`, `on=transient`. Never retry `context.Canceled`. Reset per-sample fixture counters before a retry.
4. **Project config** — load `gust.yaml` (cwd then parents). Merge **CLI > policy.yaml > gust.yaml > code defaults**. Wire plugin timeouts configurable; `default_minimum_pass_rate` applied when a scenario omits pass rate. Explicit `max_execution_error_rate: 0` is zero tolerance.
5. **Policy semantics** — `hard_constraints` counts are enforced; assertion `criticality` affects sample pass/fail and hard-fail; unmeasurable regression latency omits the ratio.
6. **Dataset CLI** — `gust dataset bundle` / `verify` with hash-checked overwrite and `--force` prune.
7. **Claim wording** — AEE/Validation generators and site docs describe golden-suite / deterministic-evaluator evidence, not field correctness of arbitrary agents.

## Out of scope

- Real LLM + real tool-call E2E → **Phase 18**
- OS-level CPU/memory/network sandboxing (docs remain honest: env scrub + timeouts + line cap)
- Stats v2, continuous eval, clustering, multi-agent (Phases 13–16)

## Dependencies

- Phase 17 semantic hardening complete
- Phase 18 should start only after this isolation/retry work
