# Live agent demo

Proves the full Gust lifecycle against a multi-step tool-calling agent — including
the **live-eval adoption path** (trigger an existing-style integration harness,
fetch traces by `{trace_id}`, evaluate, write HTML):

```text
# Direct exec (healthy / recovery / buggy / unsafe)
gust test → exec agent.py → FixtureClient → AgentRun on stdout → Wilson

# Integration / remote-QA shape (integration / integration-unsafe)
gust test → trigger harness.py → archive AgentRun under trace_id
         → fetch_trace.py {trace_id} → same assertions → Wilson + HTML
```

At 95% confidence a perfect run of 20 samples has a Wilson lower bound of ~83.9%, so it can **PASS** an 80% floor. The same N cannot PASS a 95% floor (that needs ~73 samples). Policy `on_flaky: fail` so an inconclusive interval does not look like success.

Recorded N=20 results: **[docs/usage/live-agent-demo.md](../../docs/usage/live-agent-demo.md)**.

## Run

From the repo root:

```bash
# Full suite — exec paths + integration trigger/fetch + HTML report
./demo/live-agent/run.sh

# Adoption path only (trigger IT → ingest → evaluate)
./demo/live-agent/run.sh --only integration,integration-unsafe
# or: ./demo/live-eval/run.sh

# Live Ollama
./demo/live-agent/run.sh --ollama --model llama3.2:3b
```

```powershell
.\demo\live-agent\run.ps1
.\demo\live-agent\run.ps1 -Only integration,integration-unsafe
.\demo\live-eval\run.ps1
.\demo\live-agent\run.ps1 -Ollama -Model llama3.2:3b
```

| Path | How Gust starts the sample | Trace source | Expected |
|------|----------------------------|--------------|----------|
| `healthy/` | `--runner exec` agent.py | stdout AgentRun | PASS |
| `recovery/` | exec | stdout | PASS + `error_recovery` |
| `buggy/` | exec `--buggy` | stdout | FAIL |
| `unsafe/` | exec `--unsafe` | stdout | FAIL (forbidden refund) |
| `integration/` | `--runner trigger` harness | fetch `{trace_id}` | PASS |
| `integration-unsafe/` | trigger harness `--unsafe` | fetch `{trace_id}` | FAIL |

JSON + `demo/out/live-agent-report.html` land in `demo/out/`.

| Flag | Meaning |
| --- | --- |
| `--scripted` / default | Unset `GUST_LIVE_AGENT`; deterministic tool loop |
| `--ollama` / `-Ollama` | `GUST_LIVE_AGENT=1`; real local model |
| `--samples` / `-Samples` | Wilson N (default 20) |
| `--only` / `-Only` | Subset of folders above |
| `--no-report` / `-NoReport` | Skip HTML report |
| `--skip-build` / `-SkipBuild` | Reuse `./gust` |

## Prerequisites

- Go 1.23+, Python 3.10+
- Optional: [Ollama](https://ollama.com) with `llama3.2:3b`

## Integration path (live-eval)

`integration/harness.py` stands in for *your* IT runner:

1. Gust injects sample + W3C `trace_id` / `traceparent`
2. Harness runs the same cancel-flow agent (with Gust fixtures)
3. Harness archives the `AgentRun` under `demo/out/live-agent-traces/{trace_id}.json` (stand-in for Tempo/Langfuse)
4. Gust runs `fetch_trace.py {trace_id}`, evaluates assertions, writes the HTML report

The agent under test does not print a receipt — only the harness talks to Gust’s trigger contract.
