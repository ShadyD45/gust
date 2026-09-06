#!/usr/bin/env python3
"""One-sample hook for ``gust test --runner exec`` or ``--runner http``.

    gust test testdata/scenarios/http_agent.yaml --runner exec -- python sdk/python/examples/mode3_sample.py
    python sdk/python/examples/mode3_sample.py --serve --port 8080
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from gust_sdk import FixtureClient, RunRecorder, run_sample, serve_sample  # noqa: E402


def handle(request: dict, fixtures: FixtureClient) -> RunRecorder:
    task = request.get("task") or {}
    task_input = request.get("input") or task.get("input") or "Cancel my latest order"
    task_id = task.get("id") or "refund-001"

    rec = RunRecorder(
        agent_name="support-agent",
        agent_version="1.4",
        task_id=task_id,
        task_input=task_input,
    )

    def get_orders(customer_id: int):
        if fixtures.enabled:
            return fixtures.call("get_orders", {"customer_id": customer_id})
        return [{"id": 122, "status": "DELIVERED"}, {"id": 123, "status": "PROCESSING"}]

    def cancel_order(order_id: int):
        if fixtures.enabled:
            return fixtures.call("cancel_order", {"order_id": order_id})
        return {"ok": True}

    with rec.tool("get_orders", {"customer_id": 42}) as span:
        orders = get_orders(42)
        span.output = orders

    target = next(o for o in orders if o["status"] == "PROCESSING")
    with rec.tool("cancel_order", {"order_id": target["id"]}) as span:
        span.output = cancel_order(target["id"])

    rec.complete(output=f"Order {target['id']} cancelled successfully.")
    return rec


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--serve", action="store_true", help="HTTP /invoke instead of exec stdin")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=8080)
    args = parser.parse_args()
    if args.serve:
        server = serve_sample(handle, host=args.host, port=args.port)
        print(f"listening on http://{args.host}:{args.port}/invoke", file=sys.stderr)
        try:
            server.serve_forever()
        except KeyboardInterrupt:
            server.shutdown()
        return 0
    run_sample(handle)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
