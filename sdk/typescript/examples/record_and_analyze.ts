/**
 * Record an agent run. Compile-free: run the JS equivalent or execute with
 * Node's type stripping (Node 22+).
 *
 *   node --experimental-strip-types sdk/typescript/examples/record_and_analyze.ts
 */
import { writeFileSync } from "node:fs";
import { RunRecorder } from "../src/index.js";

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
writeFileSync("out/run.json", rec.toJSON());
console.log("wrote out/run.json");
