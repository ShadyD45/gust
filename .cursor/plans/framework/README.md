# Framework Hardening Plans (Phases 10–16)

Post-MVP work that turns gust from a Go-native MVP into production-ready **test infrastructure for autonomous software** across languages and frameworks.

Read after the MVP index: [`../mvp/README.md`](../mvp/README.md) and the master [`../roadmap.md`](../roadmap.md).

Shipped from this track so far: Phase 10 file-based ingestion (`gust ingest otel`, docs in [`../../usage/otel-ingest.md`](../../usage/otel-ingest.md)) and the Phase 11 Python capture SDK ([`sdk/python/`](../../../sdk/python/)). User-facing guides live in [`docs/usage/`](../../usage/) and [`docs/extending/`](../../extending/).

## Goals

1. **Ingest** real agent traces without custom capture glue (OTel / OpenInference).
2. **Adopt** via Python/TypeScript SDKs and popular agent frameworks.
3. **Harden** statistical and policy machinery for production CI.
4. **Scale** continuous evaluation, failure mining, and multi-agent coverage.

## Phase index

| Phase | Document | Theme |
|-------|----------|--------|
| **10** | [phase-10-otel-openinference.md](phase-10-otel-openinference.md) | Trace ingestion |
| **11** | [phase-11-sdks-and-framework-adapters.md](phase-11-sdks-and-framework-adapters.md) | SDKs & adapters |
| **12** | [phase-12-calibrated-llm-judge.md](phase-12-calibrated-llm-judge.md) | Optional judge plugin |
| **13** | [phase-13-stats-regression-v2.md](phase-13-stats-regression-v2.md) | Stats / regression v2 |
| **14** | [phase-14-continuous-eval.md](phase-14-continuous-eval.md) | Production continuous eval |
| **15** | [phase-15-failure-clustering.md](phase-15-failure-clustering.md) | Failure mining |
| **16** | [phase-16-multi-agent-leaderboard.md](phase-16-multi-agent-leaderboard.md) | Multi-agent & AEE |

## Suggested sequencing

```text
Phase 10 (ingestion) ──► Phase 11 (SDKs/adapters) ──► Phase 14 (continuous eval)
         │                        │
         │                        └──► Phase 15 (clustering)
         └──► Phase 13 (stats v2) ──► Phase 12 (judge, gated)
                                      Phase 16 (leaderboard, last)
```

**Principle:** ship deterministic, offline-capable core first; add network/LLM-dependent features only behind explicit gates and calibration thresholds.
