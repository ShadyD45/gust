/**
 * Record an agent run and write AgentRun JSON for `gust analyze`.
 *
 *   npx tsx sdk/typescript/examples/record_and_analyze.ts
 */
import { mkdirSync, writeFileSync } from "node:fs";
import { RunRecorder } from "../lib/index.js";

const rec = new RunRecorder({
  agentName: "support-agent",
  agentVersion: "1.4",
  taskId: "refund-001",
  taskInput: "Cancel my latest order",
});

const orders = rec.tool("get_orders", { customer_id: 42 });
orders.output = [
  { id: 122, status: "DELIVERED" },
  { id: 123, status: "PROCESSING" },
];
orders.finish();

const cancel = rec.tool("cancel_order", { order_id: 123 });
cancel.output = { ok: true };
cancel.finish();

rec.complete("Order 123 cancelled successfully.");
mkdirSync("out", { recursive: true });
writeFileSync("out/run.json", rec.toJSON());
console.log("wrote out/run.json");
