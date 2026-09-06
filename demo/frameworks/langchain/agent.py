#!/usr/bin/env python3
"""LangChain-shaped Mode 3 hook.

Uses GustCallbackHandler when langchain-core is installed; otherwise the same
trajectory is recorded with RunRecorder so ``gust test --runner exec`` works
in CI without the extra.
"""

from __future__ import annotations

import sys
from pathlib import Path
from uuid import uuid4

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "sdk" / "python"))

from gust_sdk import FixtureClient, RunRecorder, run_sample  # noqa: E402
from gust_sdk.adapters.langchain import GustCallbackHandler  # noqa: E402


def handle(request: dict, fixtures: FixtureClient) -> RunRecorder:
    task_input = request.get("input") or "Cancel my latest order"
    rec = RunRecorder(
        agent_name="langchain-support-agent",
        agent_version="1.0",
        task_id="refund-001",
        task_input=task_input,
    )
    cb = GustCallbackHandler(recorder=rec)

    def get_orders(customer_id: int):
        if fixtures.enabled:
            return fixtures.call("get_orders", {"customer_id": customer_id})
        return [{"id": 122, "status": "DELIVERED"}, {"id": 123, "status": "PROCESSING"}]

    def cancel_order(order_id: int):
        if fixtures.enabled:
            return fixtures.call("cancel_order", {"order_id": order_id})
        return {"ok": True}

    rid = uuid4()
    cb.on_tool_start({"name": "get_orders"}, '{"customer_id": 42}', run_id=rid)
    orders = get_orders(42)
    cb.on_tool_end(orders, run_id=rid)

    target = next(o for o in orders if o["status"] == "PROCESSING")
    rid2 = uuid4()
    cb.on_tool_start({"name": "cancel_order"}, f'{{"order_id": {target["id"]}}}', run_id=rid2)
    cb.on_tool_end(cancel_order(target["id"]), run_id=rid2)

    rec.complete(output=f"Order {target['id']} cancelled successfully.")
    return rec


if __name__ == "__main__":
    run_sample(handle)
