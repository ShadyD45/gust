# Benchmarks (gust self-AEE only)

This directory holds **gust’s own** Agent Evaluation Effectiveness (AEE)
self-score. It is not a comparison suite against other agent-eval tools.

## Latest results

- **Docs site:** [Benchmarks](https://shadyd45.github.io/gust/benchmarks/) — metrics explained + [latest results](https://shadyd45.github.io/gust/benchmarks/results/)
- **Repo:** **[RESULTS.md](RESULTS.md)** — checked in manually when numbers change
- **CI runs:** [benchmark workflow](https://github.com/ShadyD45/gust/actions/workflows/benchmark.yml) — gates AEE + mock judge calibrate; writes the run’s report to the Actions job summary and uploads artifacts (does not push or open PRs)

Machine-readable JSON: [`fixtures/gust_self_aee.json`](fixtures/gust_self_aee.json).

Methodology: [`docs/usage/aee-methodology.md`](../docs/usage/aee-methodology.md) ·
[metrics explained](../docs/benchmarks/metrics.md).

## Run locally

```bash
go build -o gust ./cmd/gust
./gust aee report
./gust aee report --markdown
./gust aee report --json --out benchmarks/fixtures/gust_self_aee.json \
  --doc benchmarks/RESULTS.md \
  --site-doc docs/benchmarks/results.md
```

Commit updated `RESULTS.md` / fixture JSON / docs site results when you intentionally refresh the published numbers.

## Layout

```text
benchmarks/
  README.md
  RESULTS.md              # latest self-report (maintainer-updated)
  fixtures/
    gust_self_aee.json    # latest JSON report (maintainer-updated)
```
