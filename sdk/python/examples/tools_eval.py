"""Mode 3 eval with optional Gust fixtures when AGENTEVAL_FIXTURE_ENDPOINT is set."""

from __future__ import annotations

from gust_sdk import RunRecorder, run_sample


def handle(request, fixtures):
    task = str(request.get("input") or "")
    rec = RunRecorder(agent_name="tools-agent", agent_version="0.1", task_input=task)
    with rec.tool("lookup", {"q": task}) as span:
        if fixtures.enabled:
            span.output = fixtures.call("lookup", {"q": task})
        else:
            span.output = {"result": "live-or-fake-from-harness"}
    rec.complete(output="done")
    return rec


if __name__ == "__main__":
    run_sample(handle)
