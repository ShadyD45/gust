# Benchmarks (gust self-AEE only)

This directory holds **gust’s own** Agent Evaluation Effectiveness (AEE)
self-score. It is not a comparison suite against other agent-eval tools.

## Latest results

- **Docs site:** [Benchmarks](https://shadyd45.github.io/gust/benchmarks/) — metrics explained + [latest results](https://shadyd45.github.io/gust/benchmarks/results/)
- **Repo:** **[RESULTS.md](RESULTS.md)** — refreshed via an automated PR from the
  [`benchmark`](../.github/workflows/benchmark.yml) workflow after runs on `main`
  (so a protected default branch still works). The same PR updates
  `docs/benchmarks/results.md` for GitHub Pages.

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

## Layout

```text
benchmarks/
  README.md
  RESULTS.md              # latest self-report (CI-maintained)
  fixtures/
    gust_self_aee.json    # latest JSON report (CI-maintained)
```
