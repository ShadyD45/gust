export const SCHEMA_VERSION: "0.5";
export const INGEST_URL_ENV: "AGENTEVAL_INGEST_URL";
export const FIXTURE_ENDPOINT_ENV: "AGENTEVAL_FIXTURE_ENDPOINT";
export const SAMPLE_ID_ENV: "AGENTEVAL_SAMPLE_ID";
export const PROTOCOL_VERSION: string;

export class AgentRunError extends Error {}
export class FixtureError extends Error {}

export class SpanHandle {
  output: unknown;
  readonly spanId: string;
  setAttribute(key: string, value: unknown): void;
  finish(error?: unknown): void;
}

export class RunRecorder {
  runId: string;
  constructor(opts: {
    agentName: string;
    agentVersion: string;
    taskInput: string;
    taskId?: string;
    runId?: string;
    gitCommit?: string;
    taskContext?: Record<string, unknown>;
  });
  span(name: string, opts?: { spanType?: string; attributes?: Record<string, unknown>; parentSpanId?: string }): SpanHandle;
  tool(name: string, args?: Record<string, unknown>, parentSpanId?: string): SpanHandle;
  llm(name: string, opts?: { model?: string; parentSpanId?: string }): SpanHandle;
  recordTool(name: string, args?: Record<string, unknown>, opts?: { output?: unknown; error?: string }): string;
  complete(output?: string, durationNs?: number): void;
  fail(error: string, status?: string, output?: string): void;
  setMetadata(key: string, value: unknown): void;
  setAssertions(assertions: unknown[]): void;
  toDict(): Record<string, unknown>;
  toJSON(indent?: number): string;
  export(url?: string, timeoutMs?: number): Promise<Record<string, unknown>>;
}

export function resolveIngestUrl(url?: string): string;
export function postRun(run: Record<string, unknown>, url?: string, timeoutMs?: number): Promise<Record<string, unknown>>;

export class FixtureClient {
  endpoint: string;
  readonly enabled: boolean;
  constructor(opts?: { endpoint?: string; timeoutMs?: number });
  call(tool: string, args?: Record<string, unknown>): Promise<unknown>;
}

export class EvaluatorPlugin {
  name: string;
  version: string;
  description: string;
  capabilities?: string[];
  manifest(): Record<string, unknown>;
  evaluate(run: Record<string, unknown>, expected: unknown, context: Record<string, unknown>): { passed: boolean; score?: number; message?: string; evidence?: unknown };
  handle(request: Record<string, unknown>): Record<string, unknown>;
}

export function serve(plugin: EvaluatorPlugin, opts?: { input?: NodeJS.ReadableStream; output?: NodeJS.WritableStream }): void;
export function runSample(handler: Function, opts?: { input?: NodeJS.ReadableStream; output?: NodeJS.WritableStream }): Promise<Record<string, unknown>>;
export function serveSample(handler: Function, opts?: { host?: string; port?: number }): import("node:http").Server;
export function applySampleId(run: unknown, sampleId: string): unknown;
export function applyInvokeEnv(request: Record<string, unknown>): string;
