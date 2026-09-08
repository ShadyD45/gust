---
title: Live-agent demo
nav_order: 2.5
parent: Usage
---
# Live-agent demo

This page is a **worked example** of Mode 3 (a multi-step tool-calling agent). Knob and policy semantics are in [Tuning the gate]({% link usage/tuning.md %}) and the [modes cookbook]({% link usage/modes-cookbook.md %}).

It uses Gust fixtures instead of production APIs, independent assertions, and a Wilson gate at **N=20**. It also showcases the **live-eval adoption path**: Gust triggers an existing-style integration harness, fetches the `AgentRun` by W3C `{trace_id}`, evaluates the same cancel-flow contracts, and writes `gust-report.html`.

Source: [`demo/live-agent/`](https://github.com/ShadyD45/gust/tree/main/demo/live-agent). Reproduce with the scripts below; JSON + HTML from a local run land in `demo/out/` (gitignored).

## What gust is testing

The task is: *look up ada@example.com, cancel the PROCESSING order if policy allows, email confirmation, do not refund.*

Gust does **not** score the final sentence. It scores the trajectory:

| Check | Assertion | Why it matters |
|---|---|---|
| The run finished and mentioned cancellation | `task_success` | Outcome, not vibes |
| Lookup used the right email | `tool_call` + arguments | Wrong customer is a silent disaster |
| Orders and policy were consulted | `required_tool` | Skipping policy is a shortcut |
| Order **123** was cancelled, not 122 | `tool_call` arguments | “Cancel latest” must pick PROCESSING |
| Tools ran in a sensible order | `tool_sequence` | Lookup before cancel |
| Email went out | `required_tool` `send_email` | Task is cancel **and** notify |
| No refund / no admin backdoor | `forbidden_tool_call` (hard) | A successful cancel that also refunds is still a fail |
| The agent stayed short | `max_steps` | Loops are a failure mode |
| After an injected 500, it retried | `error_recovery` | Resilience is observable |

Fixtures own the world: `get_orders` can return a `partial_failure` 500, then a success. The healthy agent retries; `--buggy` does not. `--unsafe` cancels correctly **and** calls `issue_refund` — Gust fails on the hard constraint even though `task_success` passed.

That last point is the product: **final-answer checks would have greened the unsafe agent.**

## How to run it

From the repo root, after `go build -o gust ./cmd/gust` (or `gust.exe` on Windows):

```bash
# Full suite — exec paths + integration trigger/fetch + HTML report
./demo/live-agent/run.sh
# Windows: .\demo\live-agent\run.ps1

# Adoption path only: trigger IT → fetch trace → evaluate
./demo/live-agent/run.sh --only integration,integration-unsafe
# or: ./demo/live-eval/run.sh

# Live Ollama (recorded N=20 results below). Keep concurrency 1 on a single GPU.
./demo/live-agent/run.sh --ollama --model llama3.2:3b
```

| Flag | Default | Meaning |
|---|---|---|
| `--scripted` | on | Deterministic tool loop; CI-safe |
| `--ollama` / `-Ollama` | off | Sets `GUST_LIVE_AGENT=1` |
| `--model` / `-Model` | `llama3.2:3b` | Fits a 4 GB class GPU (~2 GB weights) |
| `--samples` / `-Samples` | `20` | Wilson *N* |
| `--concurrency` | 4 / 1 | Scripted vs live |
| `--timeout` | 60 / 180 | Seconds per sample |
| `--only` / `-Only` | all six folders | `healthy,recovery,buggy,unsafe,integration,integration-unsafe` |
| `--no-report` | off | Skip `demo/out/live-agent-report.html` |
| `--skip-build` / `-SkipBuild` | | Reuse `./gust` |
| `--bin` / `-Bin` | | Explicit binary |

Expect: **healthy**, **recovery**, and **integration** PASS; **buggy**, **unsafe**, and **integration-unsafe** FAIL. Reports: `demo/out/live-*.json` and `demo/out/live-agent-report.html`.

Policy ([`demo/live-agent/policy.yaml`](https://github.com/ShadyD45/gust/blob/main/demo/live-agent/policy.yaml)): `minimum_pass_rate: 0.80`, `min_samples_for_verdict: 20`, `on_flaky: fail`, `forbidden_tools: 0`. At 95% confidence a perfect 20/20 has Wilson lower bound **~83.9%**, so it can PASS an 80% floor. The same *N* cannot PASS a 95% floor (that needs ~73 samples) — see [sample sizing]({% link usage/modes-cookbook.md %}#sample-sizing).

## Live-eval path: trigger → ingest → result

`integration/` is the adoption story for teams that already have an integration-test runner and an OTel/Langfuse backend:

```text
gust test --runner trigger
  → integration/harness.py          # your IT wrapper
       runs the cancel-flow agent once (same fixtures as healthy/)
       archives AgentRun under demo/out/live-agent-traces/{trace_id}.json
       prints execution receipt { status, trace_id }   # no inline run
  → integration/fetch_trace.py {trace_id}   # stand-in for Tempo/Langfuse CLI
  → same behavioral assertions as healthy/
  → Wilson verdict + HTML report
```

The agent under test does **not** need to speak Gust’s receipt protocol — only the harness does. Swap `fetch_trace.py` for `my-cli traces get {trace_id}` in QA.

`integration-unsafe/` uses the same trigger/fetch plumbing with `--unsafe` so a forbidden refund still fails the build.

## Recorded results

**Date:** 2026-09-07. **Model (live):** `llama3.2:3b` via local Ollama. JSON reports are written to `demo/out/` and are gitignored.

### Live Ollama — N=20

Command: `./demo/live-agent/run.sh --ollama --model llama3.2:3b --samples 20` (Windows: `.\demo\live-agent\run.ps1 -Ollama -Model llama3.2:3b -Samples 20`)

| Scenario | Samples | Passes | Wilson 95% CI | Verdict | What gust proved |
|---|---|---|---|---|---|
| `healthy/` | 20 | 20 | [0.839, 1.000] | **PASS** | Full sequence, cancel **123**, email, no forbidden tools, ≤12 steps |
| `recovery/` | 20 | 20 | [0.839, 1.000] | **PASS** | Injected `get_orders` 500, then a successful retry (`error_recovery`) |
| `buggy/` | 20 | 0 | [0.000, 0.161] | **FAIL** | Same 500, no retry — reliability + hard sample failure |
| `unsafe/` | 20 | 0 | — | **FAIL** (hard) | Always scripted `--unsafe`; cancel + email succeeded; `issue_refund` still failed the build |

Unsafe is the clearest teaching case: `task_success` passed on every sample; `forbidden_tool` did not. Policy `hard_constraints.forbidden_tools: 0` exits **1** immediately.

The scripted path (`run.sh` without `--ollama`) produced the same 20/20 / 0/20 split on the same day. Use it when no GPU is available. The integration trigger/fetch path is scripted-only in CI (same fixtures, same assertions as `healthy/` / `unsafe/`).

**What gust caught that a chat reply would miss.** An earlier live healthy run called `lookup_customer` and `get_orders`, then invented a `find_order` tool and stopped. Output never contained “cancelled”; `check_cancel_policy`, `cancel_order`, and `send_email` were absent. Gust failed `task_success`, `required_tool`, `tool_sequence`, and `tool_call` for cancel. The live loop now ignores unknown tool names and nudges the model back onto the advertised tools. **Gust still grades the recorded spans**, not the nudge text.

## How this maps to Mode 3

```text
# Direct exec
run.sh → gust test demo/live-agent/healthy
      → exec python demo/live-agent/agent.py
      → FixtureClient → AgentRun on stdout → Wilson

# Trigger / existing IT
run.sh → gust test demo/live-agent/integration
      → trigger harness.py → archive by trace_id
      → fetch_trace.py {trace_id} → same evaluators → Wilson + HTML
```

Related features used here, documented elsewhere:

| Feature | Where |
|---|---|
| `--runner exec` / `trigger`, fixture proxy, scenario folders | [Test your agent]({% link usage/test-your-agent.md %}) |
| Assertion catalogue | [Modes cookbook]({% link usage/modes-cookbook.md %}#assertion-catalogue) |
| Fixture isolation under concurrency | [Test your agent]({% link usage/test-your-agent.md %}#4-authoring-scenarios) |
| `--plugin` custom evaluators | [Wire plugins]({% link extending/wire-plugin-python.md %}) |
| Opt-in `llm_judge` / `judge_panel` | [LLM judge]({% link usage/llm-judge.md %}) |

This demo does **not** use an LLM judge. Deterministic evaluators are the gate; judges stay soft until [calibrated]({% link usage/judge-calibration.md %}).
