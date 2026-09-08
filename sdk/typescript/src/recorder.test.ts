import assert from "node:assert/strict";
import { test } from "node:test";
import { RunRecorder, AgentRunError, resolveIngestUrl } from "./recorder.js";
import { EvaluatorPlugin } from "./wire.js";

test("records required AgentRun fields", () => {
  const rec = new RunRecorder({
    agentName: "support-agent",
    agentVersion: "1.4",
    taskInput: "Cancel my latest order",
    taskId: "refund-001",
    runId: "run-1",
  });
  rec.complete("done");
  const run = rec.toDict();
  assert.equal(run.schema_version, "0.5");
  assert.equal(run.run_id, "run-1");
  assert.equal((run.agent as { name: string }).name, "support-agent");
  assert.equal((run.outcome as { status: string }).status, "completed");
});

test("tool arguments land in attributes.input", () => {
  const rec = new RunRecorder({
    agentName: "a",
    agentVersion: "1",
    taskInput: "x",
  });
  const span = rec.tool("cancel_order", { order_id: 123 });
  span.output = { ok: true };
  span.finish();
  rec.complete("cancelled");
  const tool = (rec.toDict().trace as Record<string, unknown>[])[0];
  assert.equal(tool.type, "tool");
  assert.deepEqual((tool.attributes as { input: unknown }).input, { order_id: 123 });
});

test("complete is required before serialize", () => {
  const rec = new RunRecorder({ agentName: "a", agentVersion: "1", taskInput: "x" });
  assert.throws(() => rec.toDict(), AgentRunError);
});

test("resolveIngestUrl uses OTEL endpoint", () => {
  const prev = process.env.OTEL_EXPORTER_OTLP_ENDPOINT;
  const ingest = process.env.AGENTEVAL_INGEST_URL;
  delete process.env.AGENTEVAL_INGEST_URL;
  process.env.OTEL_EXPORTER_OTLP_ENDPOINT = "http://127.0.0.1:4318";
  try {
    assert.equal(resolveIngestUrl(), "http://127.0.0.1:4318/v1/runs");
  } finally {
    if (prev === undefined) delete process.env.OTEL_EXPORTER_OTLP_ENDPOINT;
    else process.env.OTEL_EXPORTER_OTLP_ENDPOINT = prev;
    if (ingest === undefined) delete process.env.AGENTEVAL_INGEST_URL;
    else process.env.AGENTEVAL_INGEST_URL = ingest;
  }
});

test("evaluator plugin manifest", () => {
  const plugin = new EvaluatorPlugin();
  plugin.name = "pii";
  const res = plugin.handle({ jsonrpc: "2.0", id: 1, method: "manifest" });
  assert.equal((res.result as { name: string }).name, "pii");
  assert.equal((res.result as { kind: string }).kind, "evaluator");
});
