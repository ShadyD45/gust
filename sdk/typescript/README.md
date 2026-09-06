# gust TypeScript SDK

Capture what your Node agent did as a gust `AgentRun`. Evaluation stays in the `gust` binary.

Standard library only — no runtime dependencies.

## Install

```bash
npm install gust-sdk
```

Until the package is published, use the repo path:

```bash
npm install ./sdk/typescript
```

## Record a run

```ts
import { RunRecorder } from "gust-sdk";

const rec = new RunRecorder({
  agentName: "support-agent",
  agentVersion: "1.4",
  taskId: "refund-001",
  taskInput: "Cancel my latest order",
});

const span = rec.tool("cancel_order", { order_id: 123 });
span.output = { ok: true };
span.finish();
rec.complete("Order 123 cancelled.");
await rec.export(); // AGENTEVAL_INGEST_URL or OTEL_EXPORTER_OTLP_ENDPOINT + /v1/runs
```

## Mode 3

`runSample` / `serveSample` are the one-sample hooks `gust test --runner exec|http` calls. They belong in CI or QA, not production. See [Test your agent](../../docs/usage/test-your-agent.md).

## Compatibility

| SDK version | gust schema | gust CLI |
|---|---|---|
| 0.5.x | `0.5` | 0.5.x |
