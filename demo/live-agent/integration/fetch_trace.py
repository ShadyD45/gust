#!/usr/bin/env python3
"""Fetch an AgentRun archived by the integration harness.

Stand-in for: ``my-cli traces get {trace_id}`` against Tempo/Langfuse/Phoenix.
"""

from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
TRACE_DIR = ROOT / "demo" / "out" / "live-agent-traces"


def main() -> int:
    if len(sys.argv) < 2 or not sys.argv[1].strip():
        raise SystemExit("usage: fetch_trace.py <trace_id>")
    trace_id = sys.argv[1].strip()
    path = TRACE_DIR / f"{trace_id}.json"
    if not path.is_file():
        raise SystemExit(f"trace not found: {path}")
    text = path.read_text(encoding="utf-8")
    sys.stdout.write(text if text.endswith("\n") else text + "\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
