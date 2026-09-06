---
title: Compatibility
nav_order: 6
parent: Usage
---
# Compatibility matrix

The `schema_version` an SDK writes must match the CLI. A mismatch is rejected at validation.

## SDK × schema × CLI

| SDK | Package | SDK version | gust schema | gust CLI |
|---|---|---|---|---|
| Python | `gust-sdk` | 0.5.x | `0.5` | 0.5.x |
| TypeScript | `gust-sdk` | 0.5.x | `0.5` | 0.5.x |

## Framework adapters

| Framework | Adapter | Extra | Notes |
|---|---|---|---|
| LangChain | `gust_sdk.adapters.langchain.GustCallbackHandler` | `gust-sdk[langchain]` (`langchain-core>=0.2`) | Maps `on_tool_*` / `on_llm_*` onto `RunRecorder` |
| LlamaIndex | — | — | Use `RunRecorder` at the callback manager; packaged adapter not shipped |
| AutoGen / CrewAI | — | — | Use tool wrappers; packaged adapters not shipped |
| OpenInference / OTel | `gust ingest otel` | — | Live OTLP/HTTP + gRPC (`serve` / `gust test`); file, stdin, or `--url` for offline |
| Phoenix / Arize | OTLP exporter → gust | — | Point `OTEL_EXPORTER_OTLP_ENDPOINT` at gust; no pull adapter |
| Langfuse | `gust ingest langfuse` | — | Pull one trace or session; prefer OTLP export when you own the process |
| LangSmith | — | — | Use LangSmith's OTel exporter into `gust ingest otel serve`; no pull adapter |

## Mode 3 runners

| `--runner` | Invokes | Trace collection |
|---|---|---|
| `synthetic` | nothing (seeded) | built-in `AgentRun` |
| `ollama` | local Ollama HTTP | built-in LLM span |
| `http` | `POST {endpoint}/invoke` | `response` / `file` / `otel` / `otel-file` |
| `exec` | user command | stdout / `file` / `otel` / `otel-file` |
