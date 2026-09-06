# Phase 16: Multi-Agent Evaluators and Public AEE Leaderboard

## Objectives

1. Evaluators for multi-agent **handoffs, coordination, and role adherence**.
2. Public **Agent Evaluation Effectiveness (AEE)** leaderboard — only after methodology is reproducible and numbers are actually measured.
3. Do not publish comparative marketing numbers before measurement.

## Scope

| In | Out |
|----|-----|
| New assertion types / evaluators for multi-agent graphs | Premature competitor scorecards |
| Documented AEE methodology (DR, FPR, throughput, reliability) | Hosted leaderboard SaaS (optional later) |
| Shared public fixture pack | Opaque proprietary benchmarks |

## AEE inputs (from spec)

- Detection rate & false positive rate (mutation)
- Deterministic eval throughput
- Reliability engine correctness (H7-style)
- Reproducibility of replay hashes

## Verification

- Multi-agent golden suite with known coordination bugs is detected.
- AEE report generates from CI artifacts with pinned versions.

## Exit criteria

Published methodology doc + first internal AEE score for gust itself; external comparisons only after third-party-reproducible runs.
