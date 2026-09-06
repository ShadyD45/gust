---
title: OTel ingestion
nav_order: 3
parent: Usage
---
# Ingesting OpenTelemetry / OpenInference traces

If your agent is already instrumented with OpenTelemetry — via OpenInference, OpenLLMetry, Arize Phoenix, LangSmith's OTel exporter, or your own spans — you do not need to write a recorder. Export the trace as OTLP JSON and convert it:

```bash
./gust ingest otel --file traces/otlp-export.json --output run.json
./gust analyze run.json --assertions tests/assertions.json
```

Ingestion is a pure file transform: no collector, no network, no protobuf dependency. The same export always produces the same `AgentRun`, byte for byte, so content hashes stay stable across runs and machines.

## Producing an OTLP JSON file

Any exporter that writes OTLP/HTTP JSON works. The shape gust reads is the standard export envelope:

```json
{ "resourceSpans": [ { "resource": {...}, "scopeSpans": [ { "spans": [ ... ] } ] } ] }
```

Common ways to get one:

- **OpenTelemetry Collector**: add a `file` exporter with `format: json`.
- **Python**: `OTLPSpanExporter` pointed at a local collector, or dump the spans your `SpanProcessor` sees.
- **Phoenix / Arize**: export a project's traces as OTLP JSON.

If you capture from a collector, one file may hold many traces. List them and pick one:

```bash
./gust ingest otel --file traces/otlp-export.json --list-traces
# 4bf92f3577b34da6a3ce929d0e0e4736
# 8c31d0a6b1f4471aa22e0f5d3ce77190

./gust ingest otel --file traces/otlp-export.json \
  --trace-id 4bf92f3577b34da6a3ce929d0e0e4736 --output run.json
```

Ingesting a multi-trace file without `--trace-id` is an error, not a guess. Silently picking the first trace would make the resulting gate depend on export ordering.

## Attribute mapping

Versioned mapping table (gust schema `0.5`, OpenInference conventions):

| Source | Target | Notes |
|---|---|---|
| `traceId` | `run_id` | Override with `--run-id` |
| Resource `service.name` | `agent.name` | Falls back to `unknown-agent`; override with `--agent-name` |
| Resource `service.version` | `agent.version` | Falls back to `unknown`; override with `--agent-version` |
| Resource `service.git.commit` | `agent.git_commit` | Optional |
| Root span `session.id` | `task.id` | Falls back to the trace id; override with `--task-id` |
| Root span `input.value` | `task.input` | Falls back to the root span name; override with `--task-input` |
| Root span `output.value` | `outcome.output` | |
| Root span status | `outcome.status` | `STATUS_CODE_ERROR` (or code `2`) maps to `failed`, everything else to `completed` |
| Earliest start to latest end | `outcome.duration_ns` | Whole-trace wall clock, not just the root span |
| `spanId` / `parentSpanId` | `span_id` / `parent_span_id` | Preserved verbatim |
| `tool.name` | `span.name` | Preferred over the raw span name so assertions match the logical tool |
| `tool.parameters` | `attributes.input` | JSON strings are parsed into structured values |
| `input.value` | `attributes.input` | Used when `tool.parameters` is absent |
| `output.value` | `attributes.output` | JSON strings are parsed |
| `openinference.span.kind` | `span.type` | See below |

Span kind mapping:

| OpenInference kind | gust `span.type` |
|---|---|
| `TOOL` | `tool` |
| `LLM` | `llm` |
| `RETRIEVER` | `retrieval` |
| `AGENT`, `CHAIN` | `agent` |
| `GUARDRAIL`, `EVALUATOR`, `RERANKER`, `EMBEDDING` | `agent` |
| absent, but `tool.name` or `tool.parameters` present | `tool` |
| anything else | `agent` |

That last fallback matters: assertions about tools only see spans of type `tool`, so an untagged span carrying tool attributes is still treated as a tool call rather than being silently dropped from evaluation.

## Worked example

A trace with an agent root and two tool spans ships in [`testdata/otel/openinference_cancel.json`](https://github.com/ShadyD45/gust/blob/main/testdata/otel/openinference_cancel.json):

```json
{
  "traceId": "4bf92f3577b34da6a3ce929d0e0e4736",
  "spanId": "1a2b3c4d5e6f7081",
  "parentSpanId": "00f067aa0ba902b7",
  "name": "tool.execute",
  "startTimeUnixNano": "1767225600000000000",
  "endTimeUnixNano": "1767225600010000000",
  "attributes": [
    { "key": "openinference.span.kind", "value": { "stringValue": "TOOL" } },
    { "key": "tool.name", "value": { "stringValue": "get_orders" } },
    { "key": "tool.parameters", "value": { "stringValue": "{\"customer_id\": 42}" } }
  ],
  "status": { "code": 1 }
}
```

Becomes:

```json
{
  "span_id": "1a2b3c4d5e6f7081",
  "parent_span_id": "00f067aa0ba902b7",
  "name": "get_orders",
  "type": "tool",
  "start_time": "2026-01-01T00:00:00Z",
  "end_time": "2026-01-01T00:00:00.01Z",
  "attributes": { "input": { "customer_id": 42 } },
  "status": { "code": "ok" }
}
```

Run it yourself:

```bash
./gust ingest otel --file testdata/otel/openinference_cancel.json --output out/ingested.json
./gust analyze out/ingested.json --assertions testdata/otel/assertions.json
```

```text
Analyze 4bf92f3577b34da6a3ce929d0e0e4736 → PASS (5 assertions, 182375 ns)
  ✓ task_success: task completed successfully
  ✓ tool_arguments: tool arguments matched expected specifications
  ✓ tool_arguments: tool arguments matched expected specifications
  ✓ forbidden_tool: no forbidden tool "forbidden_admin_access" was called
  ✓ tool_sequence: tool sequence satisfied
```

An ingested run is indistinguishable from a hand-authored one downstream — the same assertions pass against both, and every mode (Replay, mutation testing, scenario extraction) accepts it.

## Instrumenting for good ingestion

Ingestion quality is a function of instrumentation quality. Three attributes carry almost all the value:

1. **`openinference.span.kind = "TOOL"`** on tool spans. Without it (and without `tool.name`), tool assertions have nothing to match.
2. **`tool.parameters`** as a JSON object string. Argument assertions compare against this; a prose summary is not comparable.
3. **`input.value` / `output.value`** on the root span. These become the task input and final output that `task_success` checks.

Manual instrumentation in Python:

```python
from opentelemetry import trace

tracer = trace.get_tracer(__name__)

with tracer.start_as_current_span("tool.execute") as span:
    span.set_attribute("openinference.span.kind", "TOOL")
    span.set_attribute("tool.name", "cancel_order")
    span.set_attribute("tool.parameters", json.dumps({"order_id": order_id}))
    result = cancel_order(order_id)
    span.set_attribute("output.value", json.dumps(result))
```

## Errors are typed, never guessed

| Situation | Behavior |
|---|---|
| Empty file or zero spans | `otel: export contains no spans` |
| Unparseable JSON | `otel: malformed OTLP JSON payload` |
| Several traces, no `--trace-id` | `otel: export contains multiple traces` with the ids listed |
| `--trace-id` not present | `otel: requested trace id not present in export` |
| Span missing `spanId` or `name` | Malformed payload error naming the span |

The mapper never invents assertions, never fabricates a task input, and never marks a partially mapped trace as passing. A trace it cannot represent is an error you can act on, not a green check.

## Limits

- **File-based only.** A live OTLP receiver may land in a later release; the file path covers CI and batch analysis today.
- **One trace per run.** Multi-trace exports require explicit selection.
- **Tool I/O only.** Fixtures are not derived from ingested traces automatically — use `gust scenario from-run` on the ingested run to get those.

