"""Minimal Mode 3 eval entrypoint — existing mocks, no Gust fixture routing.

Usage:
  gust test evals/hello --runner exec --samples 5 -- python -m examples.minimal_eval
"""

from __future__ import annotations

from gust_sdk import RunRecorder, run_eval


def handle(request):
    task = str(request.get("input") or "")
    rec = RunRecorder(agent_name="hello-agent", agent_version="0.1", task_input=task)
    # Your DI / fakes would live here. Gust world_control=existing.
    rec.complete(output=f"echo: {task}")
    return rec


if __name__ == "__main__":
    run_eval(handle)
