# gust demo

End-to-end verification of the MVP Definition of Done using the synthetic runner
(no GPU / Ollama required).

## Prerequisites

- Go 1.23+
- Run from repo root

## What it runs

1. **Mutation testing** — detection rate ≥ 90%, FPR ≤ 5%
2. **Probabilistic Test** — Wilson interval + PASS on 100 perfect synthetic samples
3. **Regression compare** — flags a statistically significant pass-rate drop
4. **Analyze / Replay / Scenario extract** — Mode 1, Mode 2, and H8 extraction

## Run

```bash
# Linux / macOS — build then run (default)
./demo/run.sh

# Reuse an existing ./gust binary
./demo/run.sh --skip-build

# Point at a specific binary (CI does this)
./demo/run.sh --bin /path/to/gust

# Windows (PowerShell)
./demo/run.ps1
./demo/run.ps1 -SkipBuild
./demo/run.ps1 -Bin .\gust.exe
```

CI builds once, then runs `./demo/run.sh --bin …` to avoid a second compile (saves Actions minutes).

Artifacts under `demo/` are self-contained copies of golden inputs so the demo
does not depend on editing `testdata/`.

## Live agent (Mode 3)

The synthetic demo above proves the gust CLI. [`demo/live-agent/`](live-agent/) proves gust against a tool-calling support agent (lookup → orders → cancel policy → cancel → email).

```bash
./demo/live-agent/run.sh              # scripted, N=20 Wilson gate
./demo/live-agent/run.sh --ollama     # local llama3.2:3b
# Windows: .\demo\live-agent\run.ps1  /  .\demo\live-agent\run.ps1 -Ollama
```

Healthy and recovery must **PASS**. Buggy (no retry) and unsafe (forbidden refund) must **FAIL**. Recorded numbers and how each assertion fires: [Live-agent demo](../docs/usage/live-agent-demo.md) on the docs site.
