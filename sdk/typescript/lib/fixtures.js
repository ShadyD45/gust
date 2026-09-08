export const FIXTURE_ENDPOINT_ENV = "AGENTEVAL_FIXTURE_ENDPOINT";
export class FixtureError extends Error {
    constructor(message) {
        super(message);
        this.name = "FixtureError";
    }
}
export class FixtureClient {
    endpoint;
    timeoutMs;
    constructor({ endpoint, timeoutMs = 30000, } = {}) {
        this.endpoint = (endpoint || process.env[FIXTURE_ENDPOINT_ENV] || "").replace(/\/$/, "");
        this.timeoutMs = timeoutMs;
    }
    get enabled() {
        return Boolean(this.endpoint);
    }
    async call(tool, arguments_ = {}) {
        if (!this.enabled) {
            throw new FixtureError(`no fixture endpoint configured; set ${FIXTURE_ENDPOINT_ENV}`);
        }
        const controller = new AbortController();
        const timer = setTimeout(() => controller.abort(), this.timeoutMs);
        try {
            const resp = await fetch(`${this.endpoint}/v1/tools/call`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ tool, arguments: arguments_ }),
                signal: controller.signal,
            });
            const text = await resp.text();
            if (!resp.ok) {
                throw new FixtureError(`fixture proxy returned ${resp.status} for ${tool}: ${text}`);
            }
            const body = JSON.parse(text);
            if (body.status === "error") {
                throw new FixtureError(body.error || `fixture error for tool ${tool}`);
            }
            return body.body;
        }
        finally {
            clearTimeout(timer);
        }
    }
}
//# sourceMappingURL=fixtures.js.map