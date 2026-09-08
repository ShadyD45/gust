export const FIXTURE_ENDPOINT_ENV = "AGENTEVAL_FIXTURE_ENDPOINT" as const;

export class FixtureError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "FixtureError";
  }
}

export class FixtureClient {
  endpoint: string;
  private timeoutMs: number;

  constructor({
    endpoint,
    timeoutMs = 30000,
  }: { endpoint?: string; timeoutMs?: number } = {}) {
    this.endpoint = (endpoint || process.env[FIXTURE_ENDPOINT_ENV] || "").replace(/\/$/, "");
    this.timeoutMs = timeoutMs;
  }

  get enabled(): boolean {
    return Boolean(this.endpoint);
  }

  async call(tool: string, arguments_: Record<string, unknown> = {}): Promise<unknown> {
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
      const body = JSON.parse(text) as {
        status?: string;
        error?: string;
        body?: unknown;
      };
      if (body.status === "error") {
        throw new FixtureError(body.error || `fixture error for tool ${tool}`);
      }
      return body.body;
    } finally {
      clearTimeout(timer);
    }
  }
}
