import readline from "node:readline";
import type { Readable, Writable } from "node:stream";

export const PROTOCOL_VERSION = "1.0";

export type EvaluateResult = {
  passed: boolean;
  score?: number;
  message?: string;
  evidence?: unknown;
};

export class EvaluatorPlugin {
  name = "ts_evaluator";
  version = "0.1.0";
  description = "";
  capabilities?: string[];

  manifest(): Record<string, unknown> {
    const payload: Record<string, unknown> = {
      protocol_version: PROTOCOL_VERSION,
      kind: "evaluator",
      name: this.name,
      version: this.version,
    };
    if (this.description) payload.description = this.description;
    if (this.capabilities) payload.capabilities = [...this.capabilities];
    return payload;
  }

  evaluate(
    _run: Record<string, unknown>,
    _expected: unknown,
    _context: Record<string, unknown>,
  ): EvaluateResult {
    throw new Error("evaluate() must be implemented");
  }

  static toolSpans(run: Record<string, unknown>, name?: string): Record<string, unknown>[] {
    const trace = (run.trace as Record<string, unknown>[] | undefined) || [];
    const spans = trace.filter((s) => s.type === "tool");
    return name ? spans.filter((s) => s.name === name) : spans;
  }

  static toolArguments(span: Record<string, unknown>): Record<string, unknown> {
    const attrs = (span.attributes as Record<string, unknown> | undefined) || {};
    return (attrs.input as Record<string, unknown> | undefined) || {};
  }

  handle(request: Record<string, unknown>): Record<string, unknown> {
    const response: Record<string, unknown> = { jsonrpc: "2.0", id: request.id };
    try {
      if (request.method === "manifest") {
        response.result = this.manifest();
      } else if (request.method === "evaluate") {
        const params = (request.params as Record<string, unknown> | undefined) || {};
        const started = process.hrtime.bigint();
        const result = this.evaluate(
          (params.run as Record<string, unknown> | undefined) || {},
          params.expected,
          (params.context as Record<string, unknown> | undefined) || {},
        );
        const passed = Boolean(result.passed);
        const payload: Record<string, unknown> = {
          evaluator_name: this.name,
          evaluator_version: this.version,
          passed,
          score: result.score ?? (passed ? 1 : 0),
          execution_time_ns: Number(process.hrtime.bigint() - started),
        };
        if (result.message) payload.message = result.message;
        if (result.evidence) payload.evidence = result.evidence;
        response.result = payload;
      } else {
        response.error = { code: -32601, message: `unknown method ${request.method}` };
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      response.error = { code: -32603, message };
    }
    return response;
  }
}

export function serve(
  plugin: EvaluatorPlugin,
  {
    input = process.stdin,
    output = process.stdout,
  }: { input?: Readable; output?: Writable } = {},
): void {
  const rl = readline.createInterface({ input });
  rl.on("line", (line) => {
    const trimmed = line.trim();
    if (!trimmed) return;
    let request: Record<string, unknown>;
    try {
      request = JSON.parse(trimmed) as Record<string, unknown>;
    } catch {
      return;
    }
    output.write(JSON.stringify(plugin.handle(request)) + "\n");
  });
}
