# gust killer demo

End-to-end verification of the MVP Definition of Done using the synthetic runner
(no GPU / Ollama required).

## Prerequisites

- Go 1.23+
- Run from repo root (the scripts build `gust` for you)

## What it runs

1. **Mutation testing** — detection rate ≥ 90%, FPR ≤ 5%
2. **Probabilistic Test** — Wilson interval + PASS on 100 perfect synthetic samples
3. **Regression compare** — flags a statistically significant pass-rate drop
4. **Analyze / Replay / Scenario extract** — Mode 1, Mode 2, and H8 extraction

## Run

```bash
# Linux / macOS
./demo/run.sh

# Windows (PowerShell)
./demo/run.ps1
```

Artifacts under `demo/` are self-contained copies of golden inputs so the demo
does not depend on editing `testdata/`.
