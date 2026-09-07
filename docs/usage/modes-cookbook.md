---
title: Modes cookbook
nav_order: 2
parent: Usage
---
# Modes cookbook

Recipes for every gust command, with the flags and semantics that actually ship today. If you have not recorded an `AgentRun` yet, start with [Integrate your app]({% link usage/integrate-your-app.md %}).

| Mode | Command | Runs your agent? | Network |
|---|---|---|---|
| 1 — Analyze | `gust analyze <run.json>` | No | None |
| 2 — Replay | `gust replay <run.json>` | No (re-drives recorded I/O) | None |
| 3 — Test | `gust test <scenario.yaml\|dir>` | Yes, *N* times | Only if the runner needs it |
| — | `gust compare <baseline> <candidate>` | No | None |
| — | `gust mutate <run.json>` | No | None |
| — | `gust scenario from-run <run.json>` | No | None |
| — | `gust ingest otel serve` or `--file` / `--url` | No | None |

Every command accepts `--json` for machine-readable output. Use it in CI whenever you want to archive evidence rather than just read a terminal line.

## Mode 1: Analyze

Evaluate assertions against a captured trace. This is the cheapest gate you can run and the one that belongs on every pull request.

```bash
# Assertions from run metadata
./gust analyze testdata/runs/golden_cancel.json --policy testdata/policy.yaml

# Assertions from a separate file (recommended for real projects)
./gust analyze run.json --assertions tests/cancel_order.assertions.json

# Machine-readable evidence
./gust analyze run.json --json > report.json
```

Assertion resolution order: `--assertions <file>` wins; otherwise `metadata.assertions` inside the run; otherwise a single default `task_success` check.

### Assertion catalogue

All nine assertion types, the evaluator each one resolves to, and the fields that matter:

| `type` | Evaluator | Fields | Passes when |
|---|---|---|---|
| `task_success` | `task_success` | `parameters.expected_output` (optional substring) | `outcome.status == "completed"` and, if given, the output contains the substring |
| `tool_call` (no `arguments`) | `tool_selection` | `tool` | A span with `type: tool` and that name exists |
| `tool_call` (with `arguments`) | `tool_arguments` | `tool`, `arguments`, optional `parameters.occurrence` (`any` default, or `first` / `last` / 1-based index) | The selected occurrence(s) of that tool have `attributes.input` matching every expected key |
| `tool_sequence` | `tool_sequence` | `parameters.sequence: [names]`, optional `parameters.match` (`subsequence` default, or `exact`) | Subsequence: names appear in relative order (gaps allowed). Exact: tool names equal the sequence with no extras |
| `forbidden_tool_call` | `forbidden_tool` | `tool`, optional `arguments` | The tool was never called (or never with those arguments) |
| `required_tool` | `required_tool` | `tool` | The tool was called at least once |
| `max_steps` | `max_steps` | `limit` (default 10) | Count of top-level `tool` + `agent` spans ≤ limit (`llm` / `retrieval` spans are sub-steps and do not count) |
| `max_latency_ms` | `max_latency` | `limit` ms (default 5000), optional `parameters.latency_source: wall_clock` | `max(end) - min(start)` across the trace is within the limit (order-independent) |
| `error_recovery` | `error_recovery` | optional `parameters.after_error_tool`, `parameters.recovery_tools` | No matching error spans, or a later successful span retries the same op (or a listed recovery tool) |
| `schema_valid` | `schema_validation` | `parameters.schema` (JSON Schema for `outcome.output`) | Output must be JSON matching the declared schema; if no schema is given, only the AgentRun envelope is checked (explicitly reported) |
| `llm_judge` | `llm_judge` | `parameters.rubric`, optional threshold; requires `allow_llm_judge` | Soft, opt-in calibrated judge signal |

Two of these are **hard constraints**: a failing `forbidden_tool` or `schema_validation` fails the build immediately in Mode 3, regardless of pass rate or `on_flaky`. Set `"criticality": "hard"` to document that intent in the assertion.

A realistic assertions file:

```json
[
  { "id": "completes",     "type": "task_success", "parameters": { "expected_output": "cancelled" } },
  { "id": "reads_orders",  "type": "required_tool", "tool": "get_orders" },
  { "id": "cancels_right", "type": "tool_call", "tool": "cancel_order", "arguments": { "order_id": 123 } },
  { "id": "order_matters", "type": "tool_sequence", "parameters": { "sequence": ["get_orders", "cancel_order"] } },
  { "id": "no_refunds",    "type": "forbidden_tool_call", "tool": "issue_refund", "criticality": "hard" },
  { "id": "stays_cheap",   "type": "max_steps", "limit": 6 },
  { "id": "stays_fast",    "type": "max_latency_ms", "limit": 8000 }
]
```

Argument matching is a subset check with structured diffs: expected keys must match, extra keys in the actual call are ignored, and nested paths are reported individually in `evidence.discrepancies` so a failure tells you `order_id: expected 123, got 122` rather than "mismatch".

## Mode 2: Replay

Re-drive a recorded trajectory against fixtures. Identical inputs produce identical outputs, every time, with zero network traffic.

```bash
./gust replay testdata/runs/golden_cancel.json --fixtures testdata/fixtures
```

`--fixtures` points at a directory of `.json` files, one fixture each:

```json
{
  "fixture_id": "fx_get_orders_001",
  "tool": "get_orders",
  "match_strategy": "exact_hash",
  "recorded_input": { "customer_id": 42 },
  "recorded_response": {
    "status": "success",
    "body": [{ "id": 122, "status": "DELIVERED" }, { "id": 123, "status": "PROCESSING" }]
  },
  "provenance": "recorded"
}
```

### Matching strategies

| `match_strategy` | Behavior |
|---|---|
| `exact_hash` | Keyed by RFC 8785 canonical hash of the arguments. Order-insensitive, whitespace-insensitive, exact. |
| `ordered_sequence` | Nth call to that tool returns the Nth fixture. Use for stateful APIs where the same request must return different answers. |
| `prefer_exact_then_sequence` | Try exact hash, fall back to sequence. The default for extracted scenarios. |

You can omit `input_hash`; it is computed from `recorded_input` at load time.

### Failure injection

Set `mode` on a fixture to test what your agent does when the world misbehaves:

| `mode` | Effect |
|---|---|
| `success` (default) | Return `recorded_response` |
| `slow` | Combine with `delay_ms` to add latency before responding |
| `timeout` | Block until the context deadline expires |
| `malformed` | Return truncated, unparseable JSON |
| `partial_failure` | Return HTTP 500 with an injected server error |

```json
{
  "fixture_id": "fx_cancel_order_flaky",
  "tool": "cancel_order",
  "match_strategy": "exact_hash",
  "recorded_input": { "order_id": 123 },
  "mode": "partial_failure",
  "delay_ms": 250,
  "recorded_response": { "status": "success", "body": { "ok": true } },
  "provenance": "authored"
}
```

Pair that fixture with an `error_recovery` assertion and you have a real resilience test: does the agent retry, escalate, or silently claim success?

### Live agents against fixtures

For Mode 3 runners that call real tools, gust exposes an ephemeral HTTP mock proxy. Runners receive the endpoint and also honor the `AGENTEVAL_FIXTURE_ENDPOINT` environment variable. The contract is one route:

```http
POST /v1/tools/call
{ "tool": "get_orders", "arguments": { "customer_id": 42 } }

200 OK
{ "status": "success", "status_code": 200, "body": [ ... ] }
```

Point your agent's tool layer at that base URL in test builds and every tool call resolves from fixtures instead of production. End users: [Test your agent]({% link usage/test-your-agent.md %}). Go embedders: [Custom test runner]({% link extending/custom-test-runner.md %}).

## Mode 3: Test

Run the agent *N* times and gate on the confidence interval, not a single outcome.

```bash
./gust test testdata/scenarios/cancel_latest_order.yaml \
  --runner synthetic --samples 100 --concurrency 8 --policy testdata/policy.yaml
```

| Flag | Default | Purpose |
|---|---|---|
| `--samples` | scenario value | Override `reliability.samples` |
| `--concurrency` | `4` | Parallel workers (`gust.yaml` `test.concurrency` if flag omitted) |
| `--runner` | `synthetic` | `synthetic`, `ollama`, `http`, `exec` |
| `--endpoint` | (empty) | Agent URL for `http`; Ollama URL for `ollama` |
| `--command` | | Exec argv (or pass args after `--`) |
| `--trace-source` | `auto` | `auto` / `response` / `file` / `otel` / `otel-file` |
| `--trace-path` | | Per-sample file; may contain `{sample_id}` |
| `--otel-listen` | ephemeral | In-process OTLP/HTTP bind; gRPC is derived (`:4318` → `:4317`) |
| `--fixtures` | | Extra fixture JSON directory |
| `--model` | `llama3.1:8b` | Ollama model |
| `--pass-probability` | `1.0` | Synthetic runner pass rate — useful for testing your own gates |
| `--policy` | `gust.yaml` `policy:` or built-in defaults | Policy file |
| `--timeout` | `gust.yaml` `test.timeout` or runner default | Per-sample runner timeout (seconds) |
| `--retry-on` | `transient` | `none` / `transient` / `all` — which runner errors retry |
| `--retry-max-attempts` | `2` | Total `Run` tries including the first (`1` = no retry) |
| `--retry-backoff-ms` | `50` | Wait before a retry |
| `--max-execution-error-rate` | `0.20` | Abort as runner-unstable above this fraction; explicit `0` is zero tolerance |

Defaults merge **CLI flags > `policy.yaml` > `gust.yaml` > code defaults**. `gust init` writes a `gust.yaml` that is actually loaded (search cwd, then parents).

Retry defaults are reliability-first: only **transient** infrastructure errors retry (network/`ErrTransient`). Agent process crashes and timeouts are not retried unless you set `--retry-on all`. Each sample gets its own cloned fixtures and ephemeral mock-tool proxy so concurrent ordered fixtures do not share sequence counters.

The `synthetic` runner is seeded and needs no GPU or API key, which makes it the right choice for verifying that your policy and assertions behave before you spend tokens. `--pass-probability 0.85` lets you prove your CI actually goes red on a flaky agent.

A scenario file (or a folder — see [Authoring scenarios]({% link usage/test-your-agent.md %})):

```yaml
id: cancel_latest_order
version: "1.0"
description: "Agent should cancel the latest processing order (123), not delivered (122)."
task:
  id: refund-001
  input: "Cancel my latest order"
environment:
  fixture_strategy: prefer_exact_then_sequence
  fixtures: []
  fixtures_dir: fixtures
assertion_files:
  - ../_shared/assertions/cancel.yaml
reliability:
  samples: 100
  minimum_pass_rate: 0.95
  confidence: 0.95
provenance:
  source: authored
  extracted_at: 2026-09-06T00:00:00Z
  reviewed_by: "you@example.com"
```

```bash
./gust test testdata/suites --runner synthetic --samples 2 --policy testdata/policy-smoke.yaml
```

### Reading the verdict

The verdict compares the Wilson interval against `minimum_pass_rate`:

| Verdict | Condition | Meaning |
|---|---|---|
| `PASS` | Lower bound ≥ minimum | Proven reliable at this confidence |
| `FAIL` | Upper bound < minimum | Proven unreliable |
| `FLAKY` | Interval straddles the minimum | Inconclusive — you need more samples or a real fix |
| `INSUFFICIENT_SAMPLES` | `samples < min_samples_for_verdict` | Not enough data to say anything |

`FLAKY` is a feature, not a hedge. It is the honest answer when 97/100 cannot statistically distinguish a 95%-reliable agent from a 93%-reliable one.

### Sample sizing

Wilson lower bounds for a **perfect** run at 95% confidence:

| Samples, all passing | Lower bound | Verdict at `minimum_pass_rate: 0.95` |
|---|---|---|
| 20 | 83.9% | `FLAKY` |
| 50 | 92.9% | `FLAKY` |
| 75 | 95.1% | `PASS` |
| 100 | 96.3% | `PASS` |

The trap: at a 95% floor, **no number of samples below ~73 can ever produce `PASS`**, even with zero failures. If your gate is stuck at `FLAKY`, check your sample count before you blame the agent. Lower the floor or raise *N*.

### Policy files

```yaml
version: "1.0"
name: production-ci-gate
hard_constraints:
  forbidden_tools: 0
  schema_violations: 0
reliability:
  default_minimum_pass_rate: 0.95
  min_samples_for_verdict: 5
  on_flaky: warn        # warn | fail | ignore
regression:
  max_pass_rate_drop: 0.02
  max_latency_increase_ratio: 0.15
```

`on_flaky` is the knob that decides how strict your pipeline is:

| `on_flaky` | Exit code | OverallVerdict | Use when |
|---|---|---|---|
| `warn` | `0` | `FLAKY` | Adopting gust; visibility without blocking merges |
| `ignore` | `0` | `PASS` | Flakiness is tracked elsewhere |
| `fail` | `3` | `FLAKY` | Mature suite; inconclusive is not good enough to ship |

Exit code `3` is deliberately distinct from `1` so CI can treat "unproven" differently from "broken".

## Regression comparison

`gust compare` gates a candidate against a baseline using a two-proportion z-test, so noise does not trip the build and real drops do.

Both inputs are small JSON summaries:

```json
{ "name": "baseline", "passes": 95, "samples": 100, "pass_rate": 0.95, "latency_ns": 1000000 }
```

```bash
./gust compare baseline.json candidate.json --policy policy.yaml
```

```text
Compare → REGRESSION
  pass_rate_drop=0.1200  latency_increase_ratio=0.0500  p=0.0032 significant=true
  statistically significant pass-rate regression
```

A regression fails the build only when the drop exceeds `max_pass_rate_drop` **and** p < 0.05. A latency increase beyond `max_latency_increase_ratio` fails on its own. Store the reliability numbers from your main-branch `gust test --json` run as the baseline artifact and compare every PR against it.

## Mutation testing

Who tests the tests? `gust mutate` injects known-bad behavior into a golden run and checks that your assertions catch it.

```bash
./gust mutate testdata/runs/golden_cancel.json --classes all --n 100
```

```text
Mutation testing
  applied=9 detected=9 skipped=0
  detection_rate=100.0%  false_positive_rate=0.0%
```

The command exits `1` if detection rate < 90% or false positive rate > 5%. Run it against your own golden run and assertions: a low detection rate means your assertions are decorative, and gust will tell you so before production does.

## Scenario extraction

```bash
./gust scenario from-run production-trace.json --output tests/new_scenario.yaml
./gust scenario from-run production-trace.json --layout dir --output tests/new_case/
```

You get task input, recorded fixtures for every tool span (content-addressed by argument hash), sane reliability defaults — and this:

```yaml
assertions: []
provenance:
  source: production_trace
  source_run_id: run-2026-09-06-8f21
  reviewed_by: ""
```

Empty assertions and empty `reviewed_by` are intentional and structural. Deriving assertions from observed behavior would encode whatever the agent did — including its bugs — as the specification. A human writes the assertions and signs off.

## Trace ingestion

Already emitting OpenTelemetry spans? During a **CI job or laptop session**, point the *dev/QA* exporter at gust. Do not retarget production.

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
gust ingest otel serve --analyze --assertions tests/assertions.json
```

In CI, prefer `gust test --runner exec --trace-source otel` so the listener dies with the job. Offline: `--file`, `--file -`, or `--url`. Details: [OTel ingestion]({% link usage/otel-ingest.md %}) and [CI integration]({% link usage/ci-github-actions.md %}).

## Embedding gust as a Go library

The CLI is a thin wrapper. If you are already in Go, call the engines directly:

```go
import (
    "context"

    "gust/internal/adapters/evaluators"
    "gust/internal/core/analyze"
    "gust/internal/ports"
    "gust/pkg/api"
)

engine := analyze.NewEngine(evaluators.AllBuiltinEvaluators())
report, err := engine.AnalyzeRun(ctx, run, assertions, ports.EvaluationContext{ScenarioID: run.RunID})
if err != nil {
    return err
}
if !report.Passed {
    t.Fatalf("agent behavior regressed: %+v", report.Results)
}
```

That makes gust usable as an assertion library inside an ordinary `go test` run, with the same evaluators the CLI uses. To register your own evaluator alongside the built-ins, see [Custom evaluator]({% link extending/custom-evaluator-go.md %}).

