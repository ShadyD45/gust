"""Example harness for --runner trigger: invoke QA and return a receipt.

The agent itself does not import Gust. This process only propagates
W3C trace context and reports the known trace id back to Gust.
"""

from __future__ import annotations

import json
import os
import sys
import urllib.request

from gust_sdk.sample import apply_invoke_env, execution_receipt, read_invoke


def main() -> None:
    request = read_invoke()
    apply_invoke_env(request)
    trace_id = str(request.get("trace_id") or os.environ.get("GUST_TRACE_ID", ""))
    traceparent = str(request.get("traceparent") or os.environ.get("TRACEPARENT", ""))
    baggage = str(request.get("baggage") or os.environ.get("BAGGAGE", ""))

    qa_url = os.environ.get("QA_AGENT_URL", "").rstrip("/")
    if not qa_url:
        # Demo fallback: no QA URL configured — return a synthetic completed receipt.
        sys.stdout.write(json.dumps(execution_receipt(status="completed", trace_id=trace_id)) + "\n")
        return

    payload = json.dumps({"input": request.get("input"), "context": request.get("context") or {}}).encode("utf-8")
    req = urllib.request.Request(
        qa_url + "/invoke",
        data=payload,
        headers={
            "Content-Type": "application/json",
            "traceparent": traceparent,
            "baggage": baggage,
        },
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=60) as resp:
        _ = resp.read()
    sys.stdout.write(json.dumps(execution_receipt(status="completed", trace_id=trace_id)) + "\n")


if __name__ == "__main__":
    main()
