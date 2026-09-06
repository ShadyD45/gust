# Phase 13: Statistical Regression Engine v2

## Objectives

1. Upgrade beyond Wilson-only intervals where needed: **bootstrap CIs**, effect sizes (Cohen’s *d*), power / sample-size recommendations.
2. Improve regression comparator sensitivity/specificity for small *N*.
3. Keep closed-form Wilson as the default fast path for Mode 3.

## Scope

| In | Out |
|----|-----|
| `internal/core/stats` extensions | Breaking changes to existing verdict enums |
| Dynamic `recommend_samples(p, margin, confidence)` | Bayesian hierarchical models (later) |
| Stronger `gust compare` reporting | Replacing H7 synthetic proofs |

## Design notes

- Bootstrap must be seedable for reproducibility.
- Sample-size advisor helps users avoid “N=20 can never PASS at P_min=0.95” pitfalls (already documented).
- Effect sizes accompany pass-rate drops so CI messages cite magnitude, not only p-values.

## Verification

- Property tests: bootstrap CI covers true *p* at nominal rate on synthetic Bernoulli streams.
- Regression: tiny noise still fails to trip CI; large drops still trip.

## Exit criteria

`gust compare` and Mode 3 docs expose v2 metrics; Wilson remains default for single-scenario verdicts.
