export declare const SCHEMA_VERSION: "0.5";
export declare const INGEST_URL_ENV: "AGENTEVAL_INGEST_URL";
export declare const OTEL_ENDPOINT_ENV: "OTEL_EXPORTER_OTLP_ENDPOINT";
export type SpanType = "agent" | "llm" | "tool" | "retrieval" | "memory" | "plan" | "error";
export type OutcomeStatus = "completed" | "failed" | "timeout" | "cancelled";
export declare class AgentRunError extends Error {
    constructor(message: string);
}
type SpanRecord = {
    span_id: string;
    name: string;
    type: string;
    start_time: string;
    end_time: string;
    status: {
        code: string;
        message?: string;
    };
    parent_span_id?: string;
    attributes?: Record<string, unknown>;
};
export declare class SpanHandle {
    output: unknown;
    private _span;
    constructor(span: SpanRecord);
    get spanId(): string;
    setAttribute(key: string, value: unknown): void;
    finish(error?: unknown): void;
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
export declare class RunRecorder {
    runId: string;
    private _agent;
    private _task;
    private _spans;
    private _outcome;
    private _metadata;
    constructor({ agentName, agentVersion, taskInput, taskId, runId, gitCommit, taskContext, }: RunRecorderOptions);
    span(name: string, { spanType, attributes, parentSpanId, }?: {
        spanType?: SpanType | string;
        attributes?: Record<string, unknown>;
        parentSpanId?: string;
    }): SpanHandle;
    tool(name: string, arguments_?: Record<string, unknown>, parentSpanId?: string): SpanHandle;
    llm(name: string, { model, parentSpanId, ...attributes }?: {
        model?: string;
        parentSpanId?: string;
        [key: string]: unknown;
    }): SpanHandle;
    recordTool(name: string, arguments_?: Record<string, unknown>, { output, error }?: {
        output?: unknown;
        error?: string;
    }): string;
    complete(output?: string, durationNs?: number): void;
    fail(error: string, status?: OutcomeStatus | string, output?: string): void;
    setMetadata(key: string, value: unknown): void;
    setAssertions(assertions: unknown[]): void;
    toDict(): Record<string, unknown>;
    toJSON(indent?: number): string;
    export(url?: string, timeoutMs?: number): Promise<Record<string, unknown>>;
}
export declare function resolveIngestUrl(url?: string): string;
export declare function postRun(run: Record<string, unknown>, url?: string, timeoutMs?: number): Promise<Record<string, unknown>>;
export {};
//# sourceMappingURL=recorder.d.ts.map