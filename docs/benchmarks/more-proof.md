---
title: More proof
nav_order: 3
parent: Benchmarks
---
# More proof (beyond AEE)

AEE is the core self-score. These additional signals strengthen the claim that gust is **test infrastructure for agents** — not an answer scorer with a CI badge.

## Already shipping

| Signal | Where | What it proves |
|--------|-------|----------------|
| **Unit + package tests** | [`test` workflow](https://github.com/ShadyD45/gust/actions/workflows/test.yml) | Go core, Python SDK, TypeScript SDK stay green |
| **End-to-end smoke** | [`smoke` workflow](https://github.com/ShadyD45/gust/actions/workflows/smoke.yml) | Demo path, SDK record→analyze, Mode 3 exec |
| **Gust Validation Suite** | `gust-aee validate` in [`benchmark`](https://github.com/ShadyD45/gust/actions/workflows/benchmark.yml) | ≥50 adversarial cases with expected pass/fail/flaky/infra/mutation/fixture/recovery outcomes |
| **Mock judge calibration** | `gust judge calibrate --provider mock` in [`benchmark`](https://github.com/ShadyD45/gust/actions/workflows/benchmark.yml) | Calibration harness + Spearman ρ path works offline |
| **Hard vs soft policy** | Policy engine + docs | Forbidden tools fail immediately; soft thresholds use Wilson |
| **Trace ≠ test** | `gust scenario from-run` leaves `assertions: []` | We refuse to auto-enshrine observed bugs as the spec |

The workflows above are the source of truth for CI health.

## What we will not add

- Leaderboards vs DeepEval, Phoenix, Braintrust, etc. — different category; unfair and uninformative.
- Production agent pass-rate dashboards as a gust “benchmark” — that is *your* product quality, not suite effectiveness.
- Uncalibrated LLM-judge scores as a hard CI gate.
