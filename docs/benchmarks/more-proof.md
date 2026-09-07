---
title: More proof
nav_order: 3
parent: Benchmarks
---
# More proof (beyond AEE)

AEE is the core self-score. These additional signals are extra regression checks that the evaluation machinery and CI wiring keep working.

## Already shipping

| Signal | Where | What it checks |
|--------|-------|----------------|
|--------|-------|----------------|
| **Unit + package tests** | [`test` workflow](https://github.com/ShadyD45/gust/actions/workflows/test.yml) | Go core, Python SDK, TypeScript SDK stay green |
| **End-to-end smoke** | [`smoke` workflow](https://github.com/ShadyD45/gust/actions/workflows/smoke.yml) | Demo path, SDK record→analyze, Mode 3 exec |
| **Gust Validation Suite** | `gust-aee validate` in [`benchmark`](https://github.com/ShadyD45/gust/actions/workflows/benchmark.yml) | Curated adversarial catalog (analyze/replay/stats/fixtures/mutation/Mode 3 isolation) classified as authored |
| **Mock judge calibration** | `gust judge calibrate --provider mock` in [`benchmark`](https://github.com/ShadyD45/gust/actions/workflows/benchmark.yml) | Calibration harness + Spearman ρ path works offline |
| **Hard vs soft policy** | Policy engine + docs | Forbidden tools fail immediately; soft thresholds use Wilson |
| **Trace ≠ test** | `gust scenario from-run` leaves `assertions: []` | We refuse to auto-enshrine observed bugs as the spec |

The workflows above are the source of truth for CI health.
