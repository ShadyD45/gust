---
title: Validation Suite
nav_order: 4
parent: Benchmarks
---
# Gust Validation Suite

Published 2026-09-07 · suite `1.1.0`

A catalog of adversarial scenarios ("Should this pass/fail/be flaky/…?") that Gust must classify correctly.

| | |
|---|---|
| Cases | **83 / 83** passed |
| Gate | **PASS** |

## Category breakdown

| Category | Question theme | Passed | Total |
|---|---|---:|---:|
| `pass` | Should this pass? | 18 | 18 |
| `fail` | Should this fail? | 20 | 20 |
| `flaky` | Should this be flaky? | 3 | 3 |
| `infra` | Infrastructure error? | 4 | 4 |
| `mutation` | Mutation detected? | 18 | 18 |
| `fixture` | Fixture match? | 9 | 9 |
| `recovery` | Considered recovery? | 6 | 6 |
| `mode3` | Mode 3 sample isolation? | 5 | 5 |

Machine-readable: [`benchmarks/fixtures/gust_validation.json`](https://github.com/ShadyD45/gust/blob/main/benchmarks/fixtures/gust_validation.json).

Reproduce: `gust-aee validate`
