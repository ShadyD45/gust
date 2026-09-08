---
title: Test your agent
nav_order: 2
parent: Usage
has_mermaid: true
---
# Test your own agent (Mode 3)

You do not write Go and you do not fork Gust. Gust is a **behavioral evaluation harness**: it repeats a scenario N times, collects traces, evaluates behavioral contracts, and writes a Wilson reliability verdict plus `gust-report.html`.

New here? Start with [Getting started]({% link usage/getting-started.md %}). Product model: [What Gust is]({% link architecture/what-gust-is.md %}). Deeper context: [How Gust works]({% link architecture/how-gust-works.md %}), [Mocking]({% link architecture/mocking-and-fixtures.md %}), [Topologies]({% link architecture/execution-topologies.md %}).

## Fastest path (existing mocks, no Gust fixtures)

```bash
# Python
gust test evals/hello --runner exec --samples 5 -- python -m my_agent.gust_eval

# TypeScript
gust test evals/hello --runner exec --samples 5 -- node --import tsx my_agent/gust_eval.ts
```

### Python hook

```python
from gust_sdk import RunRecorder, run_eval

def handle(request):
    rec = RunRecorder(agent_name="my-agent", agent_version="1.0",
                      task_input=request.get("input") or "")
    # use your own DI / fakes here
    rec.complete(output="done")
    return rec

if __name__ == "__main__":
    run_eval(handle)
```

### TypeScript hook

```ts
import { RunRecorder, runEval } from "gust-sdk";

async function handle(request: { input?: string }) {
  const rec = new RunRecorder({
    agentName: "my-agent",
    agentVersion: "1.0",
    taskInput: request.input || "",
  });
  rec.complete("done");
  return rec;
}

await runEval(handle);
```

```yaml
environment:
  world_control: existing
```

Gust writes `./gust-report.html` by default (`--no-report` to skip, `--report path` to override).

If you only need to gate one captured run, use [`analyze`]({% link usage/integrate-your-app.md %}) instead. This page is for reliability sampling: *is today's live agent reliable on this scenario?*

A complete worked example with Gust fixtures — scripts, fixtures, assertions, and recorded N=20 results — is the [live-agent demo]({% link usage/live-agent-demo.md %}). Copy that folder shape when you want `world_control: gust`.

Prove the adoption features (trigger IT → fetch `{trace_id}` → evaluate → HTML) with the live-agent integration scenarios:

```bash
./demo/live-agent/run.sh --only integration,integration-unsafe
# Windows: .\demo\live-agent\run.ps1 -Only integration,integration-unsafe
```

In-tree hello evals:

```bash
# from sdk/python
gust test examples/evals/hello --runner exec --samples 5 -- \
  python -m examples.minimal_eval

# from repo root
gust test sdk/typescript/examples/evals/hello --runner exec --samples 5 -- \
  node --import tsx sdk/typescript/examples/minimal_eval.ts
```

## Where this runs (not production)

Mode 3 is a **CI (or laptop) session**. Gust is the evaluator; the agent under test is a **dev/QA** build. Production does not run Gust and does not export to it.

```mermaid
flowchart TB
  subgraph ciJob [CI job]
    gustBin[gust test]
    fixtures[Fixture proxy]
  end
  subgraph underTest [Dev or QA only]
    agent[Agent process or QA URL]
  end
  prod[Production]
  gustBin -->|start N samples| agent
  gustBin --> fixtures
  agent -->|tools| fixtures
  agent -->|OTLP or AgentRun| gustBin
  prod -.->|no gust no OTEL to CI| gustBin
```

| Topology | When to use | How traces reach Gust |
|---|---|---|
| **Same CI job** (`--runner exec`) | Default. Agent code + optional fixtures in the pipeline. | Gust injects `OTEL_*` / `AGENTEVAL_INGEST_URL` on localhost and Wait()s, or reads AgentRun from stdout |
| **QA on the same network** (`--runner http`) | Self-hosted runner or compose in the QA VPC so the agent can dial Gust | Invoke body carries `otel_endpoint`; agent exports OTLP or `POST /v1/runs` |
| **QA cannot reach the CI runner** | GitHub-hosted runners have no inbound ports | Agent returns an `AgentRun` on `/invoke` (`--trace-source response`), or CI pulls from Langfuse / `--trace-fetch-command` |
| **Existing OTel backend** (`--runner trigger`) | No Gust code in the agent; harness propagates W3C trace context | Trigger prints a receipt; Gust fetches `{trace_id}` via `--trace-fetch-command` |

Full topology matrix: [Execution topologies]({% link architecture/execution-topologies.md %}). Do not run `gust ingest otel serve` as a production sidecar. That turns Gust into a store. Full job YAML: [CI integration]({% link usage/ci-github-actions.md %}).

## 1. Instrument the agent

Record an `AgentRun` with the [Python SDK](https://github.com/ShadyD45/gust/tree/main/sdk/python), the [TypeScript SDK](https://github.com/ShadyD45/gust/tree/main/sdk/typescript), or point an existing OTel exporter at Gust — see [OTel ingestion]({% link usage/otel-ingest.md %}). For Gust-controlled tools, route selected calls through `FixtureClient` so Mode 3 never hits **production APIs**.

### Python

```python
from gust_sdk import FixtureClient, RunRecorder

fixtures = FixtureClient()  # AGENTEVAL_FIXTURE_ENDPOINT

def get_orders(customer_id):
    if fixtures.enabled:
        return fixtures.call("get_orders", {"customer_id": customer_id})
    return orders_api.list(customer_id)
```

### TypeScript

```ts
import { FixtureClient } from "gust-sdk";

const fixtures = new FixtureClient(); // AGENTEVAL_FIXTURE_ENDPOINT

async function getOrders(customerId: number) {
  if (fixtures.enabled) {
    return fixtures.call("get_orders", { customer_id: customerId });
  }
  return ordersApi.list(customerId);
}
```

## 2. Expose one sample

### Exec — Gust starts the process

**Python**

```python
from gust_sdk import run_sample

def handle(request, fixtures):
    rec = RunRecorder(agent_name="my-agent", agent_version="1.4",
                      task_input=request.get("input") or "")
    # ... run the agent once, using fixtures.call for tools ...
    rec.complete(output="done")
    return rec

if __name__ == "__main__":
    run_sample(handle)
```

**TypeScript**

```ts
import { RunRecorder, runSample } from "gust-sdk";

await runSample(async (request, fixtures) => {
  const rec = new RunRecorder({
    agentName: "my-agent",
    agentVersion: "1.4",
    taskInput: String(request.input || ""),
  });
  // ... run once; await fixtures.call(...) when world_control=gust ...
  rec.complete("done");
  return rec;
});
```

```bash
gust test tests/scenario.yaml --runner exec -- python -m my_agent.sample
gust test tests/scenario.yaml --runner exec -- node --import tsx my_agent/sample.ts
```

Gust writes the scenario JSON on stdin and sets `AGENTEVAL_FIXTURE_ENDPOINT`, `AGENTEVAL_SAMPLE_ID`, and (when the in-process receiver is up) the `OTEL_EXPORTER_OTLP_*` / `AGENTEVAL_INGEST_URL` variables. Your process can print one `AgentRun` on stdout, `export()` it, or emit OTLP.

### HTTP — Gust POSTs to a running service

```python
from gust_sdk import serve_sample
serve_sample(handle, port=8080).serve_forever()
```

```ts
import { serveSample } from "gust-sdk";
serveSample(handler, { port: 8080 });
```

```bash
gust test tests/scenario.yaml --runner http --endpoint http://localhost:8080
```

POST `/invoke` body:

```json
{
  "input": "Cancel my latest order",
  "context": {},
  "tool_endpoint": "http://127.0.0.1:49152",
  "sample_id": "cancel_latest_order-…",
  "otel_endpoint": "http://127.0.0.1:4318",
  "ingest_url": "http://127.0.0.1:4318/v1/runs"
}
```

Respond with an `AgentRun`, or write a file / export OTel and tell Gust how to collect it.

### Trigger — remote QA, no Gust import in the agent

Your harness starts the sample and prints a receipt; Gust fetches the trace by ID from your existing backend:

```bash
gust test evals/ \
  --runner trigger \
  --trace-fetch-command 'my-cli traces get {trace_id}' \
  -- -- python harness.py
```

See [OTel ingestion]({% link usage/otel-ingest.md %}) and [Execution topologies]({% link architecture/execution-topologies.md %}).

## 3. Collect the trace

| `--trace-source` | What Gust uses |
|---|---|
| `auto` (default) | Valid `AgentRun` on the HTTP body / stdout, else `{trace-path}`, else the in-process OTLP buffer |
| `response` | HTTP body or exec stdout must be an `AgentRun` |
| `file` | Read `{--trace-path}` as AgentRun JSON. Use `{sample_id}` in the path. |
| `otel-file` | Read `{--trace-path}` as OTLP JSON and map it |
| `otel` | Wait on the in-process receiver for `gust.sample_id` (or the only in-flight sample) |

`--runner exec|http` with `auto` or `otel` starts OTLP/HTTP + gRPC for you. `--otel-listen` is only needed to pin the port.

```bash
gust test tests/scenario.yaml --runner exec --trace-source otel -- python -m my_agent
gust test tests/scenario.yaml --runner http --endpoint http://localhost:8080 --trace-source otel
```

Concurrent samples cannot share one `run.json`. Stamp `AGENTEVAL_SAMPLE_ID` on `run_id` / `metadata.sample_id`, or rely on the injected `OTEL_RESOURCE_ATTRIBUTES`. The SDKs do this automatically.

## 4. Authoring scenarios

Keep the **document** (a reviewed `TestScenario`). Do not put fixture bodies and shared assertions in one YAML.

```text
tests/
  _shared/
    assertions/cancel.yaml
    policy.yaml
  cancel_latest/
    scenario.yaml          # task, refs, reliability, optional runner
    fixtures/
      fx_get_orders_001.json
      fx_cancel_order_001.json
```

`scenario.yaml` stays thin:

### Example (retail support)

```yaml
id: cancel_latest
version: "1.0"
description: "Cancel the latest PROCESSING order"
task:
  id: refund-001
  input: "Cancel my latest order"
environment:
  world_control: gust
  fixtures: []
  fixtures_dir: fixtures          # or omit; a sibling fixtures/ is loaded automatically
assertion_files:
  - ../_shared/assertions/cancel.yaml
assertions:
  - $ref: ../_shared/assertions/extra.yaml   # spliced in place
  - id: max_8
    type: max_steps
    limit: 8
reliability:
  samples: 20
  minimum_pass_rate: 0.95
  confidence: 0.95
runner:
  type: exec
  command: ["python", "-m", "my_agent.sample"]
  traces:
    source: response
```

CLI flags override `runner`. One file still works: `gust test tests/scenario.yaml`.

```bash
gust test tests/ --policy tests/_shared/policy.yaml
```

This discovers every `scenario.yaml` under the tree (skips `_shared`) and any top-level `*.yaml` that looks like a scenario. One fixture proxy and one OTel listener cover the suite; each scenario gets its own fixtures so cases cannot bleed.

Policy counts, criticality, sample size, and `on_flaky` are documented in [Tuning the gate]({% link usage/tuning.md %}).

`gust scenario from-run run.json --layout dir --output tests/new_case/` writes this folder shape with empty assertions.

`gust test` starts fixture proxies when the resolved scenario needs Gust world control (or you pass `--fixtures`). Each sample clones a clonable provider and gets an ephemeral mock-tool proxy so ordered sequences cannot interleave under concurrency. If a provider cannot be cloned, the sampler caps concurrency to 1.

No Gust process in production; the job is the listener. See [CI integration]({% link usage/ci-github-actions.md %}).

The [live-agent demo]({% link usage/live-agent-demo.md %}) is this layout in-tree (`healthy/`, `recovery/`, `buggy/`, `unsafe/`) with `run.sh` / `run.ps1` wrapping `gust test`. More YAML compositions across domains: [Scenario examples]({% link usage/examples.md %}).

## Agent failures vs infrastructure errors

| Kind | Examples | Effect |
|------|----------|--------|
| **Behavioral** | Wrong tool, failed task, forbidden call | Counts in Wilson pass rate |
| **Infrastructure** | Hook crash, invalid JSON, unreachable fixture proxy, trace fetch timeout | Excluded from Wilson; gated by max infrastructure-error rate |

Use `--fail-unattainable` when the requested PASS floor is mathematically impossible at the configured sample size.

## Embedders (Go library)

For offline Analyze inside `go test`, use the stable API — [Go library]({% link usage/go-library.md %}). Live Mode 3 stays on the CLI runners above; do not import `gust/internal/...`.
