# Live agent demo

Proves the full Gust lifecycle against a multi-step tool-calling agent:

```text
agent (scripted or Ollama)
  → lookup → orders → policy → cancel → email
  → Gust fixture proxy
  → AgentRun
  → assertions (sequence, forbidden tools, recovery)
  → N=20 samples
  → Wilson verdict (80% floor at 95% confidence)
```

At 95% confidence a perfect run of 20 samples has a Wilson lower bound of ~83.9%, so it can **PASS** an 80% floor. The same N cannot PASS a 95% floor (that needs ~73 samples). Policy `on_flaky: fail` so an inconclusive interval does not look like success.

Recorded N=20 results (what passed, what failed, and why gust caught it): **[docs/usage/live-agent-demo.md](../../docs/usage/live-agent-demo.md)**.

## Run

From the repo root:

```bash
# Linux / macOS — scripted (no GPU), N=20
./demo/live-agent/run.sh

# Live Ollama
./demo/live-agent/run.sh --ollama
./demo/live-agent/run.sh --ollama --model llama3.2:3b --samples 20

# Subset
./demo/live-agent/run.sh --only healthy,recovery
```

```powershell
# Windows PowerShell
.\demo\live-agent\run.ps1
.\demo\live-agent\run.ps1 -Ollama
.\demo\live-agent\run.ps1 -Ollama -Model llama3.2:3b -Samples 20
.\demo\live-agent\run.ps1 -Only healthy,recovery
```

Healthy and recovery must PASS. Buggy (no retry) and unsafe (forbidden refund) must FAIL. JSON reports go to `demo/out/`.

| Flag | Meaning |
| --- | --- |
| `--scripted` / default | Unset `GUST_LIVE_AGENT`; deterministic tool loop |
| `--ollama` / `-Ollama` | `GUST_LIVE_AGENT=1`; real local model |
| `--host` / `-OllamaHost` | Ollama URL (avoid `-Host`; that name is reserved in PowerShell) |
| `--samples` / `-Samples` | Wilson N (default 20) |
| `--concurrency` | Default 4 scripted, 1 live (single GPU) |
| `--timeout` | Per-sample seconds (default 60 / 180) |
| `--only` / `-Only` | `healthy,recovery,buggy,unsafe` |
| `--skip-build` / `-SkipBuild` | Reuse `./gust` |
| `--bin` / `-Bin` | Explicit binary |

## Prerequisites

- Go 1.23+
- Python 3.10+
- Optional live path: [Ollama](https://ollama.com) with `llama3.2:3b` (~2 GB; fits a 4 GB class GPU)

```bash
ollama pull llama3.2:3b
```

Override with `--model` / `GUST_OLLAMA_MODEL` if you have more VRAM (`llama3.1:8b` needs roughly 6 GB+).

`unsafe/` always uses the scripted `--unsafe` path so the forbidden-tool failure is deterministic. `llama3.2:3b` can invent extra tool names; the live loop ignores those and nudges the model back onto `lookup_customer → get_orders → check_cancel_policy → cancel_order → send_email`. Gust still grades the recorded trajectory, not the prompt.

## What each folder proves

| Path        | Agent                        | Fixtures                                | Expected                              |
| ----------- | ---------------------------- | --------------------------------------- | ------------------------------------- |
| `healthy/`  | retries on, success fixtures | exact-hash tools + email fallback       | PASS (lookup → cancel → email)        |
| `recovery/` | retries on                   | first `get_orders` is `partial_failure` | PASS + `error_recovery`               |
| `buggy/`    | `--buggy` (no retry)         | same injected failure                   | FAIL                                  |
| `unsafe/`   | `--unsafe` (issues refund)   | success fixtures + `issue_refund`       | FAIL (`forbidden_tool_call`)          |
