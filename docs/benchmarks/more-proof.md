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
| **Unit + package tests** | [`test` workflow](https://github.com/ShadyD45/gust/actions/workflows/test.yml) | Go core, Python SDK, TypeScript SDK stay green |
| **Race detector** | `go-race` job in the same workflow | Concurrent Mode 3 / fixtures |
| **Fuzz smoke** | `go-fuzz-smoke` job | AgentRun JSON, fixtures, evaluators, evidence diff, policy YAML |
| **End-to-end smoke** | [`smoke` workflow](https://github.com/ShadyD45/gust/actions/workflows/smoke.yml) | Demo path, SDK record→analyze, Mode 3 exec |
| **Gust Validation Suite** | `gust-aee validate` in [`benchmark`](https://github.com/ShadyD45/gust/actions/workflows/benchmark.yml) | Curated adversarial catalog (analyze/replay/stats/fixtures/mutation/Mode 3 isolation) classified as authored |
| **Mock judge calibration** | `gust judge calibrate --provider mock` in [`benchmark`](https://github.com/ShadyD45/gust/actions/workflows/benchmark.yml) | Calibration harness + Spearman ρ path works offline |
| **Hard vs soft policy** | Policy engine + docs | Named `hard_constraints` counts vs Wilson; soft assertions are evidence only |
| **Live-agent Mode 3** | [Live-agent demo]({% link usage/live-agent-demo.md %}) | Live `llama3.2:3b` N=20 Wilson PASS on healthy/recovery; FAIL on no-retry and forbidden-tool cases |
| **Trace ≠ test** | `gust scenario from-run` leaves `assertions: []` | We refuse to auto-enshrine observed bugs as the spec |

The workflows above are the source of truth for CI health. The live-agent suite is a **local** Mode 3 proof (not a smoke job); numbers live on that page.
