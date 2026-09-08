import type { Readable, Writable } from "node:stream";
export declare const PROTOCOL_VERSION: string;
export type EvaluateResult = {
    passed: boolean;
    score?: number;
    message?: string;
    evidence?: unknown;
};
export declare class EvaluatorPlugin {
    name: string;
    version: string;
    description: string;
    capabilities?: string[];
    manifest(): Record<string, unknown>;
    evaluate(run: Record<string, unknown>, expected: unknown, context: Record<string, unknown>): EvaluateResult;
    static toolSpans(run: Record<string, unknown>, name?: string): Record<string, unknown>[];
    static toolArguments(span: Record<string, unknown>): Record<string, unknown>;
    handle(request: Record<string, unknown>): Record<string, unknown>;
}
export declare function serve(plugin: EvaluatorPlugin, opts?: {
    input?: Readable;
    output?: Writable;
}): void;
