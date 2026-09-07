# Benchmarks (gust self-AEE + Validation Suite)

This directory holds **gust’s own** trust metrics — not a comparison suite against other agent-eval tools.

## Latest results

- **Docs site:** [Benchmarks](https://shadyd45.github.io/gust/benchmarks/) — metrics + [AEE results](https://shadyd45.github.io/gust/benchmarks/results/) + [Validation Suite](https://shadyd45.github.io/gust/benchmarks/validation/)
- **Repo:** **[RESULTS.md](RESULTS.md)** · **[VALIDATION.md](VALIDATION.md)** — checked in when numbers change
- **CI:** [benchmark workflow](https://github.com/ShadyD45/gust/actions/workflows/benchmark.yml) — gates AEE + Validation Suite + mock judge calibrate

## Run locally

```bash
go build -o gust-aee ./cmd/gust-aee

./gust-aee report
./gust-aee report --json --out benchmarks/fixtures/gust_self_aee.json \
  --doc benchmarks/RESULTS.md \
  --site-doc docs/benchmarks/results.md

./gust-aee validate
./gust-aee validate --json --out benchmarks/fixtures/gust_validation.json \
  --doc benchmarks/VALIDATION.md \
  --site-doc docs/benchmarks/validation.md
```

(`gust-aee` is internal; package managers will publish **`gust`** only — see Phase 19.)

## Layout

| Path | Contents |
|------|----------|
| `RESULTS.md` | Published AEE numbers |
| `VALIDATION.md` | Published Validation Suite summary |
| `fixtures/gust_self_aee.json` | Machine-readable AEE report |
| `fixtures/gust_validation.json` | Machine-readable Validation Suite report |

Methodology: [`docs/usage/aee-methodology.md`](../docs/usage/aee-methodology.md) ·
[metrics](../docs/benchmarks/metrics.md) ·
[validation suite](../docs/benchmarks/validation.md).
