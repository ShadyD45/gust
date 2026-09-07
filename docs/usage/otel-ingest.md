---
title: OTel ingestion
nav_order: 3
parent: Usage
has_mermaid: true
---
# Ingesting OpenTelemetry / OpenInference traces

If your agent already emits OpenTelemetry spans — OpenInference, OpenLLMetry, Arize Phoenix, LangSmith's OTel exporter, or your own — point it at gust **for the duration of a test**. You do not write a recorder and you do not dump files.

gust is not a production collector. Point **dev/QA or an in-CI process** at a short-lived listener; leave production exporters aimed at Phoenix, Langfuse, or your collector.

```bash
# laptop or CI job — not a prod sidecar
export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf

gust ingest otel serve --analyze --assertions tests/assertions.json
# in another terminal, run the *dev/QA* agent; Ctrl-C serve when you are done
```

`serve` listens on **OTLP/HTTP :4318** (`POST /v1/traces`, JSON or protobuf) and **OTLP/gRPC :4317** by default. The same `Mapper` used for files turns each export into an `AgentRun`. Add `--stdout` to print the run, or `--output-dir runs/` if you still want artifacts. Do not leave `serve` running against production traffic — that fills a directory and is not gust's job.

For Mode 3 in CI you usually do **not** run `serve` yourself. `gust test` starts the same listener in-process. See [CI integration]({% link usage/ci-github-actions.md %}).

gust-sdk users can skip OTel entirely and `POST` an `AgentRun` to `/v1/runs`:

```python
rec.complete(output="done")
rec.export()  # AGENTEVAL_INGEST_URL or http://127.0.0.1:4318/v1/runs
```

## Mode 3 — CI starts the listener

`gust test --runner exec|http` starts the same receiver in-process (HTTP + gRPC) and injects the exporter env into the child (or puts the URLs on the `/invoke` body). The job is the only place gust listens:

- `OTEL_EXPORTER_OTLP_ENDPOINT`
- `OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf`
- `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=…/v1/traces`
- `AGENTEVAL_INGEST_URL=…/v1/runs`
- `OTEL_RESOURCE_ATTRIBUTES=gust.sample_id={id}` (merged with any attributes you already set)

```bash
# Same CI job: gust + agent on localhost. Nothing in production.
gust test tests/cancel.yaml --runner exec --trace-source otel -- python -m my_agent
```

`--otel-listen` is optional. Concurrent samples must carry `gust.sample_id` (the env injection does this). A single in-flight sample is matched even if the attribute is missing. See [Test your agent]({% link usage/test-your-agent.md %}).

## Phoenix / OpenInference / a Collector

Phoenix and any OTLP pipeline already speak this protocol. For a **local or CI session**, point a *second* exporter (or a collector pipeline used only in QA) at gust. Do not retarget the production pipeline.

```bash
# QA / laptop only
export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
gust ingest otel serve --analyze --assertions tests/assertions.json
```

Jaeger, Tempo, and the OpenTelemetry Collector stay your production store. gust is an evaluation sink for a test session, not a replacement UI.

## Langfuse pull

When a trace already lives in Langfuse and you cannot re-export OTLP:

```bash
export LANGFUSE_HOST=https://cloud.langfuse.com
export LANGFUSE_PUBLIC_KEY=pk-lf-…
export LANGFUSE_SECRET_KEY=sk-lf-…

gust ingest langfuse --trace-id <id> --output run.json
# or --session <sessionId> when the session holds one trace
gust analyze run.json --assertions tests/assertions.json
```

Prefer pointing Langfuse's OTel exporter at `gust ingest otel serve` when you control the agent process.

## Offline / CI artifacts

File, stdin, and URL ingest are a pure transform: the same export always produces the same `AgentRun`, byte for byte.

```bash
gust ingest otel --file traces/otlp-export.json --output run.json
cat traces/otlp-export.json | gust ingest otel --file - --output run.json
gust ingest otel --url https://example/otlp-export.json --output run.json
gust analyze run.json --assertions tests/assertions.json
```

The shape gust reads is the standard export envelope:

```json
{ "resourceSpans": [ { "resource": {...}, "scopeSpans": [ { "spans": [ ... ] } ] } ] }
```

If one file holds many traces, list them and pick one:

```bash
gust ingest otel --file traces/otlp-export.json --list-traces
gust ingest otel --file traces/otlp-export.json \
  --trace-id 4bf92f3577b34da6a3ce929d0e0e4736 --output run.json
```

Ingesting a multi-trace file without `--trace-id` is an error, not a guess.

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
| Resource or span `gust.sample_id` | `metadata.sample_id` | Correlates live Mode 3 samples |

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

The mapping below is the shape gust produces for a TOOL span. A full OTLP export you can ingest ships in [`testdata/otel/openinference_cancel.json`](https://github.com/ShadyD45/gust/blob/main/testdata/otel/openinference_cancel.json).

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
    { "key": "tool.name", "value": { "stringValue": "lookup" } },
    { "key": "tool.parameters", "value": { "stringValue": "{\"key\": \"item-42\"}" } }
  ],
  "status": { "code": 1 }
}
```

Becomes:

```json
{
  "span_id": "1a2b3c4d5e6f7081",
  "parent_span_id": "00f067aa0ba902b7",
  "name": "lookup",
  "type": "tool",
  "start_time": "2026-01-01T00:00:00Z",
  "end_time": "2026-01-01T00:00:00.01Z",
  "attributes": { "input": { "key": "item-42" } },
  "status": { "code": "ok" }
}
```

```bash
gust ingest otel --file testdata/otel/openinference_cancel.json --output out/ingested.json
gust analyze out/ingested.json --assertions testdata/otel/assertions.json
```

```text
Analyze 4bf92f3577b34da6a3ce929d0e0e4736 → PASS (5 assertions, 182375 ns)
  ✓ task_success: task completed successfully
  ✓ tool_arguments: tool arguments matched expected specifications
  ✓ tool_arguments: tool arguments matched expected specifications
  ✓ forbidden_tool: no forbidden tool "forbidden_admin_access" was called
  ✓ tool_sequence: tool sequence satisfied
```

An ingested run is indistinguishable from a hand-authored one downstream.

## Instrumenting for good ingestion

1. **`openinference.span.kind = "TOOL"`** on tool spans. Without it (and without `tool.name`), tool assertions have nothing to match.
2. **`tool.parameters`** as a JSON object string. Argument assertions compare against this; a prose summary is not comparable.
3. **`input.value` / `output.value`** on the root span. These become the task input and final output that `task_success` checks.

```python
from opentelemetry import trace

tracer = trace.get_tracer(__name__)

with tracer.start_as_current_span("tool.execute") as span:
    span.set_attribute("openinference.span.kind", "TOOL")
    span.set_attribute("tool.name", "apply")
    span.set_attribute("tool.parameters", json.dumps({"id": item_id}))
    result = apply(item_id)
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
| Concurrent Mode 3 samples without `gust.sample_id` | Wait times out — set the resource attribute or use concurrency 1 |

The mapper never invents assertions, never fabricates a task input, and never marks a partially mapped trace as passing.

## Limits

- **One trace per mapped run.** Multi-trace exports require `--trace-id` (file) or a `gust.sample_id` (live, when concurrency > 1).
- **Tool I/O only.** Fixtures are not derived from ingested traces automatically — use `gust scenario from-run` on the ingested run to get those.
- **No observability UI.** gust stores eval artifacts (`AgentRun`, verdicts), not a span explorer. Keep Phoenix or Langfuse for that.
- **No production sidecar.** `serve` is a session on a laptop or CI runner. A standing collector next to prod is a different product.
