import readline from "node:readline";

export const PROTOCOL_VERSION = "1.0";

export class EvaluatorPlugin {
  name = "ts_evaluator";
  version = "0.1.0";
  description = "";
  capabilities;

  manifest() {
    const payload = {
      protocol_version: PROTOCOL_VERSION,
      kind: "evaluator",
      name: this.name,
      version: this.version,
    };
    if (this.description) payload.description = this.description;
    if (this.capabilities) payload.capabilities = [...this.capabilities];
    return payload;
  }

  evaluate(_run, _expected, _context) {
    throw new Error("evaluate() must be implemented");
  }

  static toolSpans(run, name) {
    const spans = (run.trace || []).filter((s) => s.type === "tool");
    return name ? spans.filter((s) => s.name === name) : spans;
  }

  static toolArguments(span) {
    return (span.attributes || {}).input || {};
  }

  handle(request) {
    const response = { jsonrpc: "2.0", id: request.id };
    try {
      if (request.method === "manifest") {
        response.result = this.manifest();
      } else if (request.method === "evaluate") {
        const params = request.params || {};
        const started = process.hrtime.bigint();
        const result = this.evaluate(params.run || {}, params.expected, params.context || {});
        const passed = Boolean(result.passed);
        response.result = {
          evaluator_name: this.name,
          evaluator_version: this.version,
          passed,
          score: result.score ?? (passed ? 1 : 0),
          execution_time_ns: Number(process.hrtime.bigint() - started),
        };
        if (result.message) response.result.message = result.message;
        if (result.evidence) response.result.evidence = result.evidence;
      } else {
        response.error = { code: -32601, message: `unknown method ${request.method}` };
      }
    } catch (err) {
      response.error = { code: -32603, message: String(err.message || err) };
    }
    return response;
  }
}

export function serve(plugin, { input = process.stdin, output = process.stdout } = {}) {
  const rl = readline.createInterface({ input });
  rl.on("line", (line) => {
    const trimmed = line.trim();
    if (!trimmed) return;
    let request;
    try {
      request = JSON.parse(trimmed);
    } catch {
      return;
    }
    output.write(JSON.stringify(plugin.handle(request)) + "\n");
  });
}
