export declare const FIXTURE_ENDPOINT_ENV: "AGENTEVAL_FIXTURE_ENDPOINT";
export declare class FixtureError extends Error {
    constructor(message: string);
}
export declare class FixtureClient {
    endpoint: string;
    constructor(opts?: {
        endpoint?: string;
        timeoutMs?: number;
    });
    get enabled(): boolean;
    call(tool: string, arguments_?: Record<string, unknown>): Promise<unknown>;
}
