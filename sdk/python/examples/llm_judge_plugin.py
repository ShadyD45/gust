#!/usr/bin/env python3
"""Tier-2 llm_judge plugin using official provider SDKs.

Install::

    pip install 'gust-sdk[judge]'   # or judge-openai / judge-anthropic / ...

Configure via env::

    export GUST_JUDGE_PROVIDER=openai   # openai|anthropic|google|ollama|generic
    export GUST_JUDGE_MODEL=gpt-4o-mini
    export OPENAI_API_KEY=...

Run::

    gust analyze run.json --policy policy-with-allow-llm-judge.yaml \\
      --judge-plugin sdk/python/examples/llm_judge_plugin.py

Assertion example::

    {
      "id": "helpful_cancel",
      "type": "llm_judge",
      "criticality": "soft",
      "parameters": {
        "rubric": "Confirm the order was cancelled in a clear, helpful tone.",
        "threshold": 0.7
      }
    }
"""

from __future__ import annotations

import sys
from pathlib import Path
from typing import Any, Dict, Optional

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from gust_sdk import EvaluatorPlugin, serve  # noqa: E402
from gust_sdk.judge import LLMJudgePlugin  # noqa: E402


class OfficialSDKLLMJudge(EvaluatorPlugin):
    name = "llm_judge"
    version = "1.0.0"
    description = "LLM judge via official provider SDKs (openai/anthropic/google/ollama/generic)"
    capabilities = ["llm_judge", "soft"]

    def __init__(self) -> None:
        self._impl = LLMJudgePlugin()

    def evaluate(
        self,
        run: Dict[str, Any],
        expected: Optional[Dict[str, Any]],
        context: Dict[str, Any],
    ) -> Dict[str, Any]:
        return self._impl.evaluate(run, expected, context)


if __name__ == "__main__":
    serve(OfficialSDKLLMJudge())
