# Phase 16: Multi-Agent Evaluators and AEE Self-Score

## Objectives

1. Evaluators for multi-agent **handoffs, coordination, and role adherence**.
2. Keep **AEE as a gust self-benchmark** (mutation DR/FPR, throughput, reproducibility, H7) — already started in Wave 1 via `gust aee report`.
3. Do **not** build peer-framework comparisons. gust is CI test infrastructure; other “eval frameworks” are a different product category unless they offer the same first-class features (replay + mutation trust + statistical Mode 3 gates).

## Scope

| In | Out |
|----|-----|
| New assertion types / evaluators for multi-agent graphs | Competitor scorecards / leaderboards vs DeepEval, Phoenix, Promptfoo, etc. |
| Hardening and publishing gust’s own AEE methodology + CI gate | Hosted comparison SaaS |
| Multi-agent golden suite for mutation / Mode 3 | Opaque proprietary cross-tool benchmarks |

## AEE inputs (self only)

- Detection rate & false positive rate (mutation)
- Deterministic eval throughput
- Reliability engine correctness (H7-style)
- Reproducibility of analyze/replay hashes

## AEE proof hardening (backlog)

Not exit criteria yet. Extend `gust aee report` / site RESULTS when ready; keep peer scoreboards out of scope.

1. **Per-mutator detection breakdown** — publish DR by mutation class so coverage is not concentrated in one easy mutant.
2. **Judge calibration on the site** — after each `benchmark` run, record mock (and optionally labeled) Spearman ρ next to AEE as a separate table; never mix into the deterministic gate.
3. **Analyze cold-start wall time** — single-process `gust analyze` latency on the golden run (p50/p95) for a human-readable CI budget claim.
4. **Assertion-type coverage matrix** — which `Assert*` types the golden suite exercises; makes evaluator gaps visible.
5. **Regression-compare noise test** — synthetic baseline vs candidate with a tiny pass-rate delta that must stay non-failing / inconclusive; proves compare does not flake CI on sampling noise.
6. **Fixture replay identity** — Mode 2: replayed tool I/O matches fixtures byte-for-byte on the golden cancel path.

Docs polish (optional): surface CI workflow badges on the docs home page and README once desired; workflows remain the source of truth.

## Verification

- Multi-agent golden suite with known coordination bugs is detected.
- `gust aee report` remains green in CI with pinned versions.

## Exit criteria

Multi-agent evaluators shipped; AEE remains a documented, CI-gated **self-score** for gust. External tool comparisons only if a peer implements the same trust stack and a fair shared methodology exists — not assumed.
