#!/usr/bin/env python3
"""Record an agent run and gate it with gust.

Run from the repository root:

    python sdk/python/examples/record_and_analyze.py --out out/run.json
    ./gust analyze out/run.json

The "agent" here is a hard-coded trajectory so the example runs anywhere.
In your own code, call ``rec.tool(...)`` from wherever tool results come back.
"""

from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from gust_sdk import FixtureClient, RunRecorder  # noqa: E402


def get_orders(fixtures: FixtureClient, customer_id: int):
    """A tool. Under test it resolves from fixtures; in production it does not."""
    if fixtures.enabled:
        return fixtures.call("get_orders", {"customer_id": customer_id})
    return [{"id": 122, "status": "DELIVERED"}, {"id": 123, "status": "PROCESSING"}]


def cancel_order(fixtures: FixtureClient, order_id: int):
    if fixtures.enabled:
        return fixtures.call("cancel_order", {"order_id": order_id})
    return {"ok": True}


def run_agent(output_path: str) -> str:
    fixtures = FixtureClient()  # reads AGENTEVAL_FIXTURE_ENDPOINT when present

    rec = RunRecorder(
        agent_name="support-agent",
        agent_version="1.4",
        task_id="refund-001",
        task_input="Cancel my latest order",
    )

    with rec.tool("get_orders", {"customer_id": 42}) as span:
        orders = get_orders(fixtures, customer_id=42)
        span.output = orders

    # The behavior under test: cancel the processing order, not the delivered one.
    target = next(o for o in orders if o["status"] == "PROCESSING")

    with rec.tool("cancel_order", {"order_id": target["id"]}) as span:
        span.output = cancel_order(fixtures, order_id=target["id"])

    rec.complete(output=f"Order {target['id']} cancelled successfully.")

    # Assertions are inlined here to keep the example self-contained. Real
    # projects keep them in their own file and pass --assertions.
    rec.set_assertions(
        [
            {"id": "a1", "type": "task_success", "parameters": {"expected_output": "cancelled"}},
            {"id": "a2", "type": "tool_call", "tool": "get_orders", "arguments": {"customer_id": 42}},
            {"id": "a3", "type": "tool_call", "tool": "cancel_order", "arguments": {"order_id": 123}},
            {"id": "a4", "type": "forbidden_tool_call", "tool": "issue_refund", "criticality": "hard"},
            {"id": "a5", "type": "max_steps", "limit": 4},
        ]
    )
    return rec.write(output_path)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", default="out/run.json", help="where to write the AgentRun")
    parser.add_argument("--gust", default="", help="path to the gust binary; runs analyze when set")
    args = parser.parse_args()

    path = run_agent(args.out)
    print(f"wrote {path}")

    if args.gust:
        return subprocess.call([args.gust, "analyze", path])
    print(f"next: ./gust analyze {path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
