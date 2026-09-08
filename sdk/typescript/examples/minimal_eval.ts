/**
 * Minimal Mode 3 eval entrypoint — existing mocks, no Gust fixture routing.
 *
 *   gust test examples/evals/hello --runner exec --samples 5 -- \
 *     node --import tsx sdk/typescript/examples/minimal_eval.ts
 */
import { RunRecorder, runEval } from "../lib/index.js";

async function handle(request: { input?: string }) {
  const task = String(request.input || "");
  const rec = new RunRecorder({
    agentName: "hello-agent",
    agentVersion: "0.1",
    taskInput: task,
  });
  // Your DI / fakes would live here. Gust world_control=existing.
  rec.complete(`echo: ${task}`);
  return rec;
}

await runEval(handle);
