import http from "node:http";
import type { Readable, Writable } from "node:stream";
import { FixtureClient } from "./fixtures.js";
import { RunRecorder } from "./recorder.js";
export declare const SAMPLE_ID_ENV: "AGENTEVAL_SAMPLE_ID";
export type SampleRequest = Record<string, unknown> & {
    input?: string;
    sample_id?: string;
    tool_endpoint?: string;
    otel_endpoint?: string;
    ingest_url?: string;
    evaluation_id?: string;
    scenario_id?: string;
    trace_id?: string;
    traceparent?: string;
    baggage?: string;
    world_control?: string;
};
export type SampleHandler = (request: SampleRequest, fixtures: FixtureClient) => RunRecorder | Record<string, unknown> | Promise<RunRecorder | Record<string, unknown>>;
export type EvalHandler = (request: SampleRequest) => RunRecorder | Record<string, unknown> | null | undefined | Promise<RunRecorder | Record<string, unknown> | null | undefined>;
export declare function applySampleId(run: RunRecorder | Record<string, unknown>, sampleId: string): RunRecorder | Record<string, unknown>;
export declare function applyInvokeEnv(request: SampleRequest): string;
export declare function sampleContext(): Record<string, string>;
export declare function executionReceipt(opts?: {
    status?: string;
    trace_id?: string;
    run_id?: string;
    run?: Record<string, unknown>;
    error?: string;
}): Record<string, unknown>;
export declare function runSample(handler: SampleHandler, opts?: {
    input?: Readable;
    output?: Writable;
}): Promise<Record<string, unknown>>;
export declare function runEval(handler: EvalHandler, opts?: {
    input?: Readable;
    output?: Writable;
}): Promise<Record<string, unknown>>;
export declare function serveSample(handler: SampleHandler, opts?: {
    host?: string;
    port?: number;
}): http.Server;
