# Phase 10: OpenTelemetry / OpenInference Ingestion

## Status

File-based ingestion is **shipped**: `internal/adapters/ingest/otel` maps OTLP JSON exports to `AgentRun`, exposed as `gust ingest otel --file … [--trace-id …] [--list-traces]`. Golden fixture in `testdata/otel/`, user documentation in [`docs/usage/otel-ingest.md`](../../usage/otel-ingest.md).

Remaining in this phase: the live OTLP HTTP/gRPC receiver.

## Objectives

1. Ingest agent traces from **OpenTelemetry** and **OpenInference** semantic conventions into `AgentRun`.
2. Feed Analyze mode without requiring a proprietary capture format.
3. Preserve content addressing and offline evaluation once traces are mapped.

## Scope

| In | Out |
|----|-----|
| OTLP HTTP/gRPC receiver (or file/OTLP dump importer) | Full observability backend / UI |
| Mapping OpenInference span attrs → `api.Span` | Changing core Evaluate APIs |
| CLI: `gust ingest otel --file …` / `--endpoint …` | Live sampling (Phase 14) |

## Package layout

```text
internal/adapters/ingest/
  otel/
    mapper.go       # OTel/OpenInference → AgentRun
    receiver.go     # optional OTLP listener
  ingest_test.go
cmd path: gust ingest …
```

## Design notes

- Prefer **stdlib + thin protobuf/OTLP** only if unavoidable; otherwise start with JSON OTLP export files for zero-dep MVP path.
- Mapping must be documented as a versioned table (OpenInference attr → gust field).
- Invalid/partial traces produce typed errors; never silently invent assertions.

## Verification

- Golden OTel/OpenInference fixtures map to stable `AgentRun` hashes.
- `gust analyze` on ingested runs matches hand-authored JSON runs for the same trajectory.
- No network required when ingesting from files.

## Exit criteria

Analyze mode accepts production-shaped traces from at least one OTel exporter path end-to-end.
