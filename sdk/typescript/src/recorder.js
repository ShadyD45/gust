export const SCHEMA_VERSION = "0.5";
export const INGEST_URL_ENV = "AGENTEVAL_INGEST_URL";
export const OTEL_ENDPOINT_ENV = "OTEL_EXPORTER_OTLP_ENDPOINT";

const SPAN_TYPES = new Set(["agent", "llm", "tool", "retrieval", "memory", "plan", "error"]);
const OUTCOME_STATUSES = new Set(["completed", "failed", "timeout", "cancelled"]);

export class AgentRunError extends Error {
  constructor(message) {
    super(message);
    this.name = "AgentRunError";
  }
}

function now() {
  return new Date().toISOString().replace(/\.\d+Z$/, "Z");
}

export class SpanHandle {
  constructor(span) {
    this._span = span;
    this.output = undefined;
  }

  get spanId() {
    return this._span.span_id;
  }

  setAttribute(key, value) {
    this._span.attributes = this._span.attributes || {};
    this._span.attributes[key] = value;
  }

  finish(error) {
    this._span.end_time = now();
    if (this.output !== undefined) {
      this._span.attributes = this._span.attributes || {};
      this._span.attributes.output = this.output;
    }
    if (error) {
      this._span.status = { code: "error", message: String(error.message || error) };
    }
  }
}

export class RunRecorder {
  constructor({ agentName, agentVersion, taskInput, taskId, runId, gitCommit, taskContext } = {}) {
    if (!agentName || !agentVersion) {
      throw new AgentRunError("agentName and agentVersion are required");
    }
    if (!taskInput) {
      throw new AgentRunError("taskInput is required");
    }
    const envSample = process.env.AGENTEVAL_SAMPLE_ID || "";
    this.runId = runId || envSample || crypto.randomUUID();
    this._agent = { name: agentName, version: agentVersion };
    if (gitCommit) this._agent.git_commit = gitCommit;
    this._task = { id: taskId || this.runId, input: taskInput };
    if (taskContext) this._task.context = taskContext;
    this._spans = [];
    this._outcome = null;
    this._metadata = {};
    if (envSample) this._metadata.sample_id = envSample;
  }

  span(name, { spanType = "agent", attributes, parentSpanId } = {}) {
    if (!SPAN_TYPES.has(spanType)) {
      throw new AgentRunError(`unknown span type ${spanType}`);
    }
    if (!name) throw new AgentRunError("span name is required");
    const span = {
      span_id: `s${this._spans.length + 1}`,
      name,
      type: spanType,
      start_time: now(),
      end_time: now(),
      status: { code: "ok" },
    };
    if (parentSpanId) span.parent_span_id = parentSpanId;
    if (attributes) span.attributes = { ...attributes };
    this._spans.push(span);
    return new SpanHandle(span);
  }

  tool(name, arguments_ = {}, parentSpanId) {
    return this.span(name, { spanType: "tool", attributes: { input: { ...arguments_ } }, parentSpanId });
  }

  llm(name, { model, parentSpanId, ...attributes } = {}) {
    const attrs = { ...attributes };
    if (model) attrs.model = model;
    return this.span(name, { spanType: "llm", attributes: attrs, parentSpanId });
  }

  recordTool(name, arguments_, { output, error } = {}) {
    const handle = this.tool(name, arguments_);
    handle.output = output;
    handle.finish(error ? new Error(error) : undefined);
    return handle.spanId;
  }

  complete(output = "", durationNs) {
    this._outcome = { status: "completed", output };
    if (durationNs !== undefined) this._outcome.duration_ns = durationNs;
  }

  fail(error, status = "failed", output = "") {
    if (!OUTCOME_STATUSES.has(status)) {
      throw new AgentRunError(`unknown outcome status ${status}`);
    }
    this._outcome = { status, error, output };
  }

  setMetadata(key, value) {
    this._metadata[key] = value;
  }

  setAssertions(assertions) {
    this._metadata.assertions = assertions;
  }

  toDict() {
    if (!this._outcome) {
      throw new AgentRunError("run has no outcome; call complete() or fail() before serializing");
    }
    const run = {
      schema_version: SCHEMA_VERSION,
      run_id: this.runId,
      agent: this._agent,
      task: this._task,
      trace: this._spans,
      outcome: this._outcome,
    };
    if (Object.keys(this._metadata).length) run.metadata = this._metadata;
    return run;
  }

  toJSON(indent = 2) {
    return JSON.stringify(this.toDict(), null, indent);
  }

  async export(url, timeoutMs = 10000) {
    return postRun(this.toDict(), url, timeoutMs);
  }
}

export function resolveIngestUrl(url) {
  let dest = (url || process.env[INGEST_URL_ENV] || "").trim();
  if (!dest) {
    const otel = (process.env[OTEL_ENDPOINT_ENV] || "").trim();
    if (otel) dest = otel.replace(/\/$/, "") + "/v1/runs";
  }
  if (!dest) {
    throw new AgentRunError(
      "no ingest URL; pass url or set AGENTEVAL_INGEST_URL / OTEL_EXPORTER_OTLP_ENDPOINT"
    );
  }
  dest = dest.replace(/\/$/, "");
  if (!dest.endsWith("/v1/runs")) dest = dest + "/v1/runs";
  return dest;
}

export async function postRun(run, url, timeoutMs = 10000) {
  const dest = resolveIngestUrl(url);
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), timeoutMs);
  try {
    const resp = await fetch(dest, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(run),
      signal: ctrl.signal,
    });
    if (!resp.ok) {
      const detail = await resp.text();
      throw new AgentRunError(`export ${dest}: HTTP ${resp.status}: ${detail}`);
    }
  } finally {
    clearTimeout(timer);
  }
  return run;
}
