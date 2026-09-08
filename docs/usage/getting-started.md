---
title: Getting started
nav_order: 0.5
parent: Usage
---
# Getting started with Gust

Goal: gate your agent’s **behavior** (tools, order, safety, success) in under 15 minutes — no Gust service to deploy, no agent rewrite.

## 1. Install the CLI

```bash
git clone https://github.com/ShadyD45/gust && cd gust
go build -o gust ./cmd/gust
# Windows: go build -o gust.exe ./cmd/gust
```

Optional SDKs (record traces / Mode 3 hooks):

```bash
pip install -e sdk/python
# or
npm install ./sdk/typescript
```

## 2. Pick your first path

| Path | When | Next |
|------|------|------|
| **A. Offline gate** | You already have a captured run JSON | [Analyze one run](#a-offline-gate-one-captured-run) |
| **B. Live eval (recommended)** | You can start the agent once from a script | [Live evaluation](#b-live-evaluation-mode-3) |
| **C. Worked demo** | You want a proven in-tree run first | [Live-eval adoption demo](#c-worked-demos) |

## A. Offline gate (one captured run)

```bash
./gust analyze testdata/runs/golden_cancel.json --policy testdata/policy.yaml
echo $?   # 0 = pass, 1 = fail
```

### Record a run — Python

```python
from gust_sdk import RunRecorder

rec = RunRecorder(agent_name="my-agent", agent_version="1.0",
                  task_input="Cancel my latest order")
with rec.tool("cancel_order", {"order_id": 123}) as span:
    span.output = {"ok": True}
rec.complete(output="cancelled")
rec.write("run.json")
```

### Record a run — TypeScript

```ts
import { writeFileSync } from "node:fs";
import { RunRecorder } from "gust-sdk";

const rec = new RunRecorder({
  agentName: "my-agent",
  agentVersion: "1.0",
  taskInput: "Cancel my latest order",
});
const span = rec.tool("cancel_order", { order_id: 123 });
span.output = { ok: true };
span.finish();
rec.complete("cancelled");
writeFileSync("run.json", rec.toJSON());
```

```bash
./gust analyze run.json --assertions assertions.json
```

More: [Integrate your app]({% link usage/integrate-your-app.md %}).

## B. Live evaluation (Mode 3)

Gust repeats a scenario N times against **your** agent process, scores behavioral contracts, and writes `./gust-report.html`.

### Minimal scenario

```yaml
# evals/hello/scenario.yaml
id: hello
version: "1.0"
task:
  id: hello-001
  input: "say hello"
environment:
  world_control: existing   # keep your own mocks / DI
assertions:
  - id: completes
    type: task_success
reliability:
  samples: 5
  minimum_pass_rate: 0.6
  confidence: 0.95
```

### Minimal agent hook — Python

```python
# my_agent/gust_eval.py
from gust_sdk import RunRecorder, run_eval

def handle(request):
    rec = RunRecorder(agent_name="my-agent", agent_version="1.0",
                      task_input=request.get("input") or "")
    # call your agent once; use your existing fakes here
    rec.complete(output=f"echo: {request.get('input') or ''}")
    return rec

if __name__ == "__main__":
    run_eval(handle)
```

```bash
./gust test evals/hello --runner exec --samples 5 -- python -m my_agent.gust_eval
```

### Minimal agent hook — TypeScript

```ts
// gust_eval.ts
import { RunRecorder, runEval } from "gust-sdk";

async function handle(request: { input?: string }) {
  const rec = new RunRecorder({
    agentName: "my-agent",
    agentVersion: "1.0",
    taskInput: request.input || "",
  });
  rec.complete(`echo: ${request.input || ""}`);
  return rec;
}

await runEval(handle);
```

```bash
./gust test evals/hello --runner exec --samples 5 -- node --import tsx gust_eval.ts
```

### Run the in-tree hello evals

```bash
# Python (from sdk/python so the examples package resolves)
cd sdk/python
../../gust test examples/evals/hello --runner exec --samples 5 -- \
  python -m examples.minimal_eval

# TypeScript (from repo root; needs: npm i -D tsx)
./gust test sdk/typescript/examples/evals/hello --runner exec --samples 5 -- \
  node --import tsx sdk/typescript/examples/minimal_eval.ts
```

### What “good” looks like

- Terminal shows Wilson interval + verdict (`PASS` / `FAIL` / `FLAKY` / …)
- `./gust-report.html` opens with summary + failed/excluded samples
- Agent failures count toward Wilson; crashed hooks / bad JSON are infrastructure errors (gated separately)

## C. Worked demos

Prove Mode 3 on the retail cancel agent — including trigger → fetch-by-trace → HTML:

```bash
# Adoption path: Gust triggers an IT harness, ingests traces, evaluates
./demo/live-agent/run.sh --only integration,integration-unsafe
# Windows: .\demo\live-agent\run.ps1 -Only integration,integration-unsafe

# Full suite (exec + integration)
./demo/live-agent/run.sh
```

Expect: `integration` **PASS**, `integration-unsafe` **FAIL**; HTML at `demo/out/live-agent-report.html`.

Full walkthrough (healthy / recovery / buggy / unsafe + recorded N=20): [Live-agent demo]({% link usage/live-agent-demo.md %}).

## D. Next steps by need

| Need | Guide |
|------|--------|
| Adoption smoke demo (trigger→ingest) | [`demo/live-agent/` integration](https://github.com/ShadyD45/gust/tree/main/demo/live-agent/integration) |
| Fixtures, forbidden tools, recovery | [Live-agent demo]({% link usage/live-agent-demo.md %}) |
| HTTP agent, OTel, remote QA trigger | [Test your agent]({% link usage/test-your-agent.md %}) |
| Copy-paste domain scenarios | [Scenario examples]({% link usage/examples.md %}) |
| Policy / sample size / CI knobs | [Tuning the gate]({% link usage/tuning.md %}) |
| GitHub Actions | [CI integration]({% link usage/ci-github-actions.md %}) |
| Product model (what Gust is / isn’t) | [What Gust is]({% link architecture/what-gust-is.md %}) |

**Rule of thumb:** a captured trace is evidence, not a test. Assertions are authored by you — Gust will not invent them from a golden run.
