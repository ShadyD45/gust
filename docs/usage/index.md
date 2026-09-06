---
title: Usage
nav_order: 2
has_children: true
has_toc: false
---
# Using gust

gust answers one question about an LLM agent you already run in production:

> **Does it still do the right thing — reliably — after this change?**

Not "is the final answer good?" but "did it call the right tools, with the right arguments, in the right order, without touching anything forbidden, within its step and latency budget — and how often out of *N* attempts?"

## What you get

| Problem you have today | What gust gives you |
|---|---|
| A prompt/model/tool change silently broke tool selection | `gust analyze` gates captured traces offline in CI, no LLM calls |
| Tests hit real APIs, so CI is slow, flaky, and expensive | `gust replay` re-drives recorded tool I/O deterministically |
| One test run says PASS, the next says FAIL | `gust test` samples *N* times and reports a Wilson confidence interval with `PASS` / `FAIL` / `FLAKY` / `INSUFFICIENT_SAMPLES` |
| "Is this drop real or noise?" | `gust compare` gates a baseline vs candidate pass-rate drop with a significance test |
| Production broke in a way no test covered | `gust scenario from-run` turns a real trace into a test scenario skeleton |
| "Do our assertions actually catch bugs?" | `gust mutate` injects known-bad behavior and measures detection rate |

## Guides

| Page | What it covers |
|------|----------------|
| [Integrate your app]({% link usage/integrate-your-app.md %}) | Emit an `AgentRun` from your existing Python, TypeScript, or Go service and run your first gate. Start here. |
| [Modes cookbook]({% link usage/modes-cookbook.md %}) | Recipes per mode, all assertion types, fixtures, and policies |
| [OTel ingestion]({% link usage/otel-ingest.md %}) | Convert OpenTelemetry / OpenInference spans instead of writing a recorder |
| [CI integration]({% link usage/ci-github-actions.md %}) | Wire the gate into CI with the right exit codes |

Custom evaluators, runners, and cross-language plugins: [Extending]({% link extending/index.md %}).

## The one rule that surprises people

**A trace is not a test.** A captured `AgentRun` is evidence of what happened once. If you auto-generate assertions from what the agent did, you enshrine its current bugs as the specification. gust deliberately refuses to do this — `gust scenario from-run` leaves `assertions: []` for a human to fill in.

## Install

```bash
git clone https://github.com/your-org/gust && cd gust
go build -o gust ./cmd/gust
```

The result is a single static binary with no runtime dependencies. Drop it on a CI runner or in a container; it needs no network unless you use Test mode against a live agent.

## The 60-second version

```bash
# You have a captured run and some assertions about it
./gust analyze testdata/runs/golden_cancel.json --policy testdata/policy.yaml

# Exit code 0 = gate passed, 1 = gate failed. That's your CI step.
echo $?
```

Everything else in these docs is a variation on that: where the run comes from, how many times you sample it, and what policy decides "pass".

