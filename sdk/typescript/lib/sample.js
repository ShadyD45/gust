import http from "node:http";
import { FixtureClient, FIXTURE_ENDPOINT_ENV } from "./fixtures.js";
import { INGEST_URL_ENV, OTEL_ENDPOINT_ENV, RunRecorder, postRun } from "./recorder.js";
export const SAMPLE_ID_ENV = "AGENTEVAL_SAMPLE_ID";
const RESOURCE_ATTRS_ENV = "OTEL_RESOURCE_ATTRIBUTES";
const SAMPLE_ID_ATTR = "gust.sample_id";
export function applySampleId(run, sampleId) {
    if (!sampleId)
        return run;
    if (run instanceof RunRecorder) {
        run.runId = sampleId;
        run.setMetadata("sample_id", sampleId);
        return run;
    }
    run.run_id = sampleId;
    run.metadata = { ...(run.metadata || {}), sample_id: sampleId };
    return run;
}
function asDict(run, sampleId) {
    const stamped = applySampleId(run, sampleId);
    return stamped instanceof RunRecorder ? stamped.toDict() : stamped;
}
export function applyInvokeEnv(request) {
    const sampleId = String(request.sample_id || process.env[SAMPLE_ID_ENV] || "");
    if (sampleId)
        process.env[SAMPLE_ID_ENV] = sampleId;
    for (const [key, envName] of [
        ["evaluation_id", "GUST_EVALUATION_ID"],
        ["scenario_id", "GUST_SCENARIO_ID"],
        ["trace_id", "GUST_TRACE_ID"],
        ["traceparent", "TRACEPARENT"],
        ["baggage", "BAGGAGE"],
        ["world_control", "GUST_WORLD_CONTROL"],
    ]) {
        const value = request[key];
        if (value)
            process.env[envName] = String(value);
    }
    const endpoint = String(request.tool_endpoint || process.env[FIXTURE_ENDPOINT_ENV] || "");
    if (endpoint)
        process.env[FIXTURE_ENDPOINT_ENV] = endpoint;
    const otel = String(request.otel_endpoint || process.env[OTEL_ENDPOINT_ENV] || "").replace(/\/$/, "");
    let ingest = String(request.ingest_url || process.env[INGEST_URL_ENV] || "");
    if (otel) {
        if (!process.env[OTEL_ENDPOINT_ENV])
            process.env[OTEL_ENDPOINT_ENV] = otel;
        if (!process.env.OTEL_EXPORTER_OTLP_PROTOCOL)
            process.env.OTEL_EXPORTER_OTLP_PROTOCOL = "http/protobuf";
        if (!process.env.OTEL_EXPORTER_OTLP_TRACES_ENDPOINT) {
            process.env.OTEL_EXPORTER_OTLP_TRACES_ENDPOINT = otel + "/v1/traces";
        }
        if (!ingest)
            ingest = otel + "/v1/runs";
    }
    if (ingest && !process.env[INGEST_URL_ENV])
        process.env[INGEST_URL_ENV] = ingest;
    if (sampleId) {
        const existing = process.env[RESOURCE_ATTRS_ENV] || "";
        const token = `${SAMPLE_ID_ATTR}=${sampleId}`;
        if (!existing.split(",").includes(token)) {
            process.env[RESOURCE_ATTRS_ENV] = existing ? `${existing},${token}` : token;
        }
    }
    return sampleId;
}
export function sampleContext() {
    return {
        evaluation_id: process.env.GUST_EVALUATION_ID || "",
        scenario_id: process.env.GUST_SCENARIO_ID || "",
        sample_id: process.env[SAMPLE_ID_ENV] || "",
        trace_id: process.env.GUST_TRACE_ID || "",
        traceparent: process.env.TRACEPARENT || "",
        baggage: process.env.BAGGAGE || "",
        world_control: process.env.GUST_WORLD_CONTROL || "",
        tool_endpoint: process.env[FIXTURE_ENDPOINT_ENV] || "",
        input: process.env.AGENTEVAL_TASK_INPUT || "",
    };
}
export function executionReceipt({ status = "completed", trace_id = "", run_id = "", run, error = "" } = {}) {
    const out = { status };
    if (trace_id)
        out.trace_id = trace_id;
    if (run_id)
        out.run_id = run_id;
    if (error)
        out.error = error;
    if (run)
        out.run = run;
    return out;
}
async function maybeExport(run) {
    if (process.env[INGEST_URL_ENV]) {
        await postRun(run);
    }
}
export async function runSample(handler, { input, output } = {}) {
    const chunks = [];
    const stream = input || process.stdin;
    if (stream !== process.stdin || !stream.isTTY) {
        for await (const chunk of stream)
            chunks.push(Buffer.from(chunk));
    }
    const raw = Buffer.concat(chunks).toString("utf8").trim();
    const request = raw ? JSON.parse(raw) : {};
    const sampleId = applyInvokeEnv(request);
    const endpoint = String(request.tool_endpoint || process.env[FIXTURE_ENDPOINT_ENV] || "");
    const fixtures = new FixtureClient({ endpoint });
    const run = asDict(await handler(request, fixtures), sampleId);
    await maybeExport(run);
    const sink = output || process.stdout;
    sink.write(JSON.stringify(run) + "\n");
    return run;
}
/** Minimal eval entrypoint when world_control is existing (your mocks/DI). */
export async function runEval(handler, opts) {
    return runSample(async (request) => {
        const result = await handler(request);
        if (result == null) {
            const rec = new RunRecorder({
                agentName: process.env.GUST_AGENT_NAME || "agent",
                agentVersion: process.env.GUST_AGENT_VERSION || "0",
                taskInput: String(request.input || ""),
            });
            rec.complete("traced-externally");
            return rec;
        }
        return result;
    }, opts);
}
export function serveSample(handler, { host = "127.0.0.1", port = 8080 } = {}) {
    const server = http.createServer(async (req, res) => {
        if (req.method !== "POST" || (req.url !== "/invoke" && req.url !== "/")) {
            res.writeHead(404);
            res.end();
            return;
        }
        const chunks = [];
        for await (const chunk of req)
            chunks.push(Buffer.from(chunk));
        let request = {};
        try {
            request = JSON.parse(Buffer.concat(chunks).toString("utf8") || "{}");
        }
        catch {
            res.writeHead(400);
            res.end("invalid JSON");
            return;
        }
        if (!request.sample_id) {
            request.sample_id = String(req.headers["x-gust-sample-id"] || process.env[SAMPLE_ID_ENV] || "");
        }
        const sampleId = applyInvokeEnv(request);
        const endpoint = String(request.tool_endpoint || process.env[FIXTURE_ENDPOINT_ENV] || "");
        const fixtures = new FixtureClient({ endpoint });
        try {
            const run = asDict(await handler(request, fixtures), sampleId);
            await maybeExport(run);
            const body = JSON.stringify(run);
            res.writeHead(200, { "Content-Type": "application/json", "Content-Length": Buffer.byteLength(body) });
            res.end(body);
        }
        catch (err) {
            res.writeHead(500);
            res.end(String(err instanceof Error ? err.message : err));
        }
    });
    server.listen(port, host);
    return server;
}
