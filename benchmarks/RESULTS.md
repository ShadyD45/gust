# AEE self-report (latest)

This file is **updated by CI** via a pull request from the `benchmark` workflow
(so a protected `main` branch still works). Do not edit the metrics tables by
hand - re-run `gust aee report` instead.

## gust AEE self-report

**Gate:** **PASS** | **Score:** `1.000` | **Generated:** `2026-09-06T19:22:43Z`

### Metrics

| Metric | Value | Gate |
| --- | --- | --- |
| Detection rate | **100.0%** (16/16 mutants) | >= 90% |
| False positive rate | **0.0%** | <= 5% |
| Eval throughput | **10.5M** cases/sec | >= 1000 |
| Reproducibility | **100%** identical (20 trials) | 100% |

### Reliability engine (H7)

| Vector | Result |
| --- | --- |
| PASS case | `100/100 -> PASS` |
| FLAKY case | `20/20 -> FLAKY` |
| FAIL case | `17/20 -> FAIL` |
| Overall | **all correct** |

---

_Self-benchmark only - methodology: [`docs/usage/aee-methodology.md`](../docs/usage/aee-methodology.md)._

## Reproduce locally

```bash
go build -o gust ./cmd/gust
./gust aee report
./gust aee report --json --out benchmarks/fixtures/gust_self_aee.json
./gust aee report --doc benchmarks/RESULTS.md --site-doc docs/benchmarks/results.md
```
