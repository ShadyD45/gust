# Shared AEE fixtures

Optional place for exported `gust aee report --json` outputs.

CI writes [`gust_self_aee.json`](gust_self_aee.json) on every successful `benchmark` run on `main`.

```bash
./gust aee report --json --out benchmarks/fixtures/gust_self_aee.json \
  --doc benchmarks/RESULTS.md
```
