# gust TypeScript SDK

Capture what your Node/TypeScript agent did as a gust `AgentRun`. Evaluation stays in the `gust` binary.

**Source is TypeScript** (`src/`). Published entrypoints are compiled ESM in `lib/` (regenerate with `npm run build`). Standard library only at runtime — no production dependencies.

## Install

```bash
npm install gust-sdk
```

Until the package is published, use the repo path:

```bash
npm install ./sdk/typescript
# regenerates lib/ if TypeScript is available
```

## Record a run

```ts
import { writeFileSync } from "node:fs";
import { RunRecorder } from "gust-sdk";

const rec = new RunRecorder({
  agentName: "my-agent",
  agentVersion: "1.4",
  taskId: "task-001",
  taskInput: "Complete the assigned item",
});

const span = rec.tool("apply", { id: 123 });
span.output = { ok: true };
span.finish();
rec.complete("Item 123 applied.");
writeFileSync("run.json", rec.toJSON());
// or: await rec.export(); // AGENTEVAL_INGEST_URL or OTEL_EXPORTER_OTLP_ENDPOINT + /v1/runs
```

## Live evaluation (Mode 3)

`runEval` is the minimal hook for `gust test --runner exec` when you keep your own mocks (`world_control: existing`):

```ts
import { RunRecorder, runEval } from "gust-sdk";

async function handle(request: { input?: string }) {
  const rec = new RunRecorder({
    agentName: "my-agent",
    agentVersion: "1.0",
    taskInput: request.input || "",
  });
  // your DI / fakes here
  rec.complete(`echo: ${request.input || ""}`);
  return rec;
}

await runEval(handle);
```

```bash
gust test evals/hello --runner exec --samples 5 -- node --import tsx examples/minimal_eval.ts
```

Use `runSample` / `serveSample` when Gust fixtures (`FixtureClient`) should mock selected tools. See [Getting started](../../docs/usage/getting-started.md) and [Test your agent](../../docs/usage/test-your-agent.md).

## Develop

```bash
npm install
npm run build   # src/*.ts → lib/*.js + .d.ts
npm test
```

## Compatibility

| SDK version | gust schema | gust CLI |
|---|---|---|
| 0.5.x | `0.5` | 0.5.x |
