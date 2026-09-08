export const SCHEMA_VERSION = "0.5" as const;
export const INGEST_URL_ENV = "AGENTEVAL_INGEST_URL" as const;
export const OTEL_ENDPOINT_ENV = "OTEL_EXPORTER_OTLP_ENDPOINT" as const;

const SPAN_TYPES = new Set(["agent", "llm", "tool", "retrieval", "memory", "plan", "error"]);
const OUTCOME_STATUSES = new Set(["completed", "failed", "timeout", "cancelled"]);

export type SpanType = "agent" | "llm" | "tool" | "retrieval" | "memory" | "plan" | "error";
export type OutcomeStatus = "completed" | "failed" | "timeout" | "cancelled";

export class AgentRunError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "AgentRunError";
  }
}

function now(): string {
  return new Date().toISOString().replace(/\.\d+Z$/, "Z");
}

type SpanRecord = {
  span_id: string;
  name: string;
  type: string;
  start_time: string;
  end_time: string;
  status: { code: string; message?: string };
  parent_span_id?: string;
  attributes?: Record<string, unknown>;
};

export class SpanHandle {
  output: unknown;
  private _span: SpanRecord;

  constructor(span: SpanRecord) {
    this._span = span;
    this.output = undefined;
  }

  get spanId(): string {
    return this._span.span_id;
  }

  setAttribute(key: string, value: unknown): void {
    this._span.attributes = this._span.attributes || {};
    this._span.attributes[key] = value;
  }

  finish(error?: unknown): void {
    this._span.end_time = now();
    if (this.output !== undefined) {
      this._span.attributes = this._span.attributes || {};
      this._span.attributes.output = this.output;
    }
    if (error) {
      const message =
        error instanceof Error ? error.message : String(error);
      this._span.status = { code: "error", message };
    }
  }
}

export type RunRecorderOptions = {
  agentName: string;
  agentVersion: string;
  taskInput: string;
  taskId?: string;
  runId?: string;
  gitCommit?: string;
  taskContext?: Record<string, unknown>;
};

export class RunRecorder {
  runId: string;
  private _agent: { name: string; version: string; git_commit?: string };
  private _task: { id: string; input: string; context?: Record<string, unknown> };
  private _spans: SpanRecord[];
  private _outcome: Record<string, unknown> | null;
  private _metadata: Record<string, unknown>;

  constructor({
    agentName,
    agentVersion,
    taskInput,
    taskId,
    runId,
    gitCommit,
    taskContext,
  }: RunRecorderOptions) {
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

  span(
    name: string,
    {
      spanType = "agent",
      attributes,
      parentSpanId,
    }: {
      spanType?: SpanType | string;
      attributes?: Record<string, unknown>;
      parentSpanId?: string;
    } = {},
  ): SpanHandle {
    if (!SPAN_TYPES.has(spanType)) {
      throw new AgentRunError(`unknown span type ${spanType}`);
    }
    if (!name) throw new AgentRunError("span name is required");
    const span: SpanRecord = {
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

  tool(
    name: string,
    arguments_: Record<string, unknown> = {},
    parentSpanId?: string,
  ): SpanHandle {
    return this.span(name, {
      spanType: "tool",
      attributes: { input: { ...arguments_ } },
      parentSpanId,
    });
  }

  llm(
    name: string,
    {
      model,
      parentSpanId,
      ...attributes
    }: { model?: string; parentSpanId?: string; [key: string]: unknown } = {},
  ): SpanHandle {
    const attrs: Record<string, unknown> = { ...attributes };
    if (model) attrs.model = model;
    return this.span(name, { spanType: "llm", attributes: attrs, parentSpanId });
  }

  recordTool(
    name: string,
    arguments_: Record<string, unknown> = {},
    { output, error }: { output?: unknown; error?: string } = {},
  ): string {
    const handle = this.tool(name, arguments_);
    handle.output = output;
    handle.finish(error ? new Error(error) : undefined);
    return handle.spanId;
  }

  complete(output = "", durationNs?: number): void {
    this._outcome = { status: "completed", output };
    if (durationNs !== undefined) this._outcome.duration_ns = durationNs;
  }

  fail(error: string, status: OutcomeStatus | string = "failed", output = ""): void {
    if (!OUTCOME_STATUSES.has(status)) {
      throw new AgentRunError(`unknown outcome status ${status}`);
    }
    this._outcome = { status, error, output };
  }

  setMetadata(key: string, value: unknown): void {
    this._metadata[key] = value;
  }

  setAssertions(assertions: unknown[]): void {
    this._metadata.assertions = assertions;
  }

  toDict(): Record<string, unknown> {
    if (!this._outcome) {
      throw new AgentRunError("run has no outcome; call complete() or fail() before serializing");
    }
    const run: Record<string, unknown> = {
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

  toJSON(indent = 2): string {
    return JSON.stringify(this.toDict(), null, indent);
  }

  async export(url?: string, timeoutMs = 10000): Promise<Record<string, unknown>> {
    return postRun(this.toDict(), url, timeoutMs);
  }
}

export function resolveIngestUrl(url?: string): string {
  let dest = (url || process.env[INGEST_URL_ENV] || "").trim();
  if (!dest) {
    const otel = (process.env[OTEL_ENDPOINT_ENV] || "").trim();
    if (otel) dest = otel.replace(/\/$/, "") + "/v1/runs";
  }
  if (!dest) {
    throw new AgentRunError(
      "no ingest URL; pass url or set AGENTEVAL_INGEST_URL / OTEL_EXPORTER_OTLP_ENDPOINT",
    );
  }
  dest = dest.replace(/\/$/, "");
  if (!dest.endsWith("/v1/runs")) dest = dest + "/v1/runs";
  return dest;
}

export async function postRun(
  run: Record<string, unknown>,
  url?: string,
  timeoutMs = 10000,
): Promise<Record<string, unknown>> {
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
