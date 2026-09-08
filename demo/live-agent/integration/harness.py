#!/usr/bin/env python3
"""Existing integration-test harness for Gust ``--runner trigger``.

This is the adoption story:

1. Gust starts one sample and calls *this* harness (not the agent binary).
2. The harness runs the same cancel-flow check your IT suite already owns.
3. The harness archives the AgentRun under the W3C ``trace_id`` (stand-in for
   Tempo / Langfuse / Phoenix).
4. Gust fetches that trace via ``fetch_trace.py {trace_id}`` and evaluates it.

The agent under test does not need to know Gust is driving the sample — only
the harness sees sample/trace context.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
LIVE = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "sdk" / "python"))
sys.path.insert(0, str(LIVE))

from gust_sdk import FixtureClient  # noqa: E402
from gust_sdk.sample import (  # noqa: E402
    apply_invoke_env,
    apply_sample_id,
    execution_receipt,
    read_invoke,
)

import agent as live_agent  # noqa: E402

TRACE_DIR = ROOT / "demo" / "out" / "live-agent-traces"


def run_integration_case(request: dict, *, buggy: bool, unsafe: bool) -> dict:
    """One IT case: drive the live-agent once and return an AgentRun dict."""
    sample_id = apply_invoke_env(request)
    endpoint = request.get("tool_endpoint") or os.environ.get("AGENTEVAL_FIXTURE_ENDPOINT") or ""
    fixtures = FixtureClient(endpoint=endpoint if endpoint else None)
    rec = live_agent.handle_factory(buggy=buggy, unsafe=unsafe)(request, fixtures)
    run = apply_sample_id(rec, sample_id)
    return run.to_dict() if hasattr(run, "to_dict") else run


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--buggy", action="store_true")
    parser.add_argument("--unsafe", action="store_true")
    args = parser.parse_args()

    request = read_invoke()
    trace_id = str(request.get("trace_id") or os.environ.get("GUST_TRACE_ID", "")).strip()
    if not trace_id:
        raise SystemExit("missing trace_id — Gust trigger must inject sample context")

    run = run_integration_case(request, buggy=args.buggy, unsafe=args.unsafe)

    TRACE_DIR.mkdir(parents=True, exist_ok=True)
    (TRACE_DIR / f"{trace_id}.json").write_text(json.dumps(run), encoding="utf-8")

    # Receipt only — Gust must fetch the archived AgentRun (remote-QA shape).
    sys.stdout.write(json.dumps(execution_receipt(status="completed", trace_id=trace_id)) + "\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
