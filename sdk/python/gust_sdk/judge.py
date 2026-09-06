"""LLM judges backed by official provider SDKs.

Install extras as needed::

    pip install 'gust-sdk[judge]'           # openai + anthropic + google-genai + ollama
    pip install 'gust-sdk[judge-openai]'    # openai only
    pip install 'gust-sdk[judge-anthropic]'
    pip install 'gust-sdk[judge-google]'
    pip install 'gust-sdk[judge-ollama]'

Then run as a Tier-2 plugin (overrides the Go generic adapter)::

    gust analyze run.json --policy policy.yaml \\
      --judge-plugin sdk/python/examples/llm_judge_plugin.py

Environment:

- ``GUST_JUDGE_PROVIDER`` — openai | anthropic | google | ollama | generic (default openai)
- ``GUST_JUDGE_MODEL`` — model id
- Provider API keys as documented by each official SDK
"""

from __future__ import annotations

import json
import os
import re
import time
from dataclasses import dataclass
from typing import Any, Dict, Optional, Protocol


JUDGE_SYSTEM = (
    "You are an evaluation judge for an autonomous agent. "
    "Score how well the agent satisfied the rubric on a scale from 0.0 to 1.0. "
    'Respond with ONLY a single JSON object (no markdown): '
    '{"score": <number 0-1>, "passed": <bool>, "rationale": "<short reason>"}'
)


@dataclass
class JudgeResult:
    score: float
    passed: bool
    rationale: str = ""
    model: str = ""
    raw: str = ""


class JudgeClient(Protocol):
    """Minimal protocol implemented by each official-SDK wrapper."""

    name: str

    def judge(
        self,
        *,
        rubric: str,
        run: Dict[str, Any],
        threshold: float = 0.7,
        few_shot: str = "",
        model: Optional[str] = None,
    ) -> JudgeResult: ...


def build_user_prompt(
    rubric: str,
    run: Dict[str, Any],
    *,
    threshold: float = 0.7,
    few_shot: str = "",
) -> str:
    task = (run.get("task") or {}).get("input") or ""
    outcome = run.get("outcome") or {}
    lines = [
        "Rubric:",
        rubric,
        "",
    ]
    if few_shot:
        lines.extend(["Examples:", few_shot, ""])
    lines.extend(
        [
            "Task input:",
            str(task),
            "",
            f"Agent outcome status: {outcome.get('status', '')}",
            "Agent output:",
            str(outcome.get("output") or ""),
            "",
            "Trace summary:",
        ]
    )
    for span in run.get("trace") or []:
        status = (span.get("status") or {}).get("code", "")
        lines.append(f"- [{span.get('type')}] {span.get('name')} ({status})")
    lines.append(f"\nPass threshold: {threshold:.2f}\n")
    return "\n".join(lines)


def parse_judge_content(content: str, model: str, threshold: float) -> JudgeResult:
    raw = (content or "").strip()
    match = re.search(r"\{.*\}", raw, flags=re.DOTALL)
    blob = match.group(0) if match else raw
    try:
        data = json.loads(blob)
        score = float(data.get("score", 0.0))
        rationale = str(data.get("rationale") or "")
        passed = data.get("passed")
        if passed is None:
            passed = score >= threshold
        else:
            passed = bool(passed)
    except (json.JSONDecodeError, TypeError, ValueError):
        try:
            score = float(raw)
            rationale = ""
            passed = score >= threshold
        except ValueError as exc:
            raise ValueError(f"could not parse judge output: {raw!r}") from exc
    score = max(0.0, min(1.0, score))
    return JudgeResult(
        score=score,
        passed=bool(passed),
        rationale=rationale,
        model=model,
        raw=raw,
    )


class OpenAIJudge:
    """Wrapper around the official ``openai`` Python SDK."""

    name = "openai"

    def __init__(self, model: Optional[str] = None, api_key: Optional[str] = None, base_url: Optional[str] = None):
        try:
            from openai import OpenAI
        except ImportError as exc:
            raise ImportError("Install openai: pip install 'gust-sdk[judge-openai]'") from exc
        kwargs: Dict[str, Any] = {}
        if api_key:
            kwargs["api_key"] = api_key
        if base_url:
            kwargs["base_url"] = base_url
        self._client = OpenAI(**kwargs)
        self.model = model or os.getenv("GUST_JUDGE_MODEL") or "gpt-4o-mini"

    def judge(
        self,
        *,
        rubric: str,
        run: Dict[str, Any],
        threshold: float = 0.7,
        few_shot: str = "",
        model: Optional[str] = None,
    ) -> JudgeResult:
        model_id = model or self.model
        resp = self._client.chat.completions.create(
            model=model_id,
            temperature=0,
            messages=[
                {"role": "system", "content": JUDGE_SYSTEM},
                {
                    "role": "user",
                    "content": build_user_prompt(rubric, run, threshold=threshold, few_shot=few_shot),
                },
            ],
        )
        content = resp.choices[0].message.content or ""
        return parse_judge_content(content, model_id, threshold)


class AnthropicJudge:
    """Wrapper around the official ``anthropic`` Python SDK."""

    name = "anthropic"

    def __init__(self, model: Optional[str] = None, api_key: Optional[str] = None):
        try:
            import anthropic
        except ImportError as exc:
            raise ImportError("Install anthropic: pip install 'gust-sdk[judge-anthropic]'") from exc
        self._client = anthropic.Anthropic(api_key=api_key) if api_key else anthropic.Anthropic()
        self.model = model or os.getenv("GUST_JUDGE_MODEL") or "claude-3-5-haiku-latest"

    def judge(
        self,
        *,
        rubric: str,
        run: Dict[str, Any],
        threshold: float = 0.7,
        few_shot: str = "",
        model: Optional[str] = None,
    ) -> JudgeResult:
        model_id = model or self.model
        msg = self._client.messages.create(
            model=model_id,
            max_tokens=512,
            system=JUDGE_SYSTEM,
            messages=[
                {
                    "role": "user",
                    "content": build_user_prompt(rubric, run, threshold=threshold, few_shot=few_shot),
                }
            ],
        )
        content = ""
        for block in msg.content:
            text = getattr(block, "text", None)
            if text:
                content += text
        return parse_judge_content(content, model_id, threshold)


class GoogleJudge:
    """Wrapper around the official ``google-genai`` Python SDK."""

    name = "google"

    def __init__(self, model: Optional[str] = None, api_key: Optional[str] = None):
        try:
            from google import genai
        except ImportError as exc:
            raise ImportError("Install google-genai: pip install 'gust-sdk[judge-google]'") from exc
        key = api_key or os.getenv("GOOGLE_API_KEY") or os.getenv("GEMINI_API_KEY") or os.getenv("GOOGLE_GENAI_API_KEY")
        self._client = genai.Client(api_key=key) if key else genai.Client()
        self.model = model or os.getenv("GUST_JUDGE_MODEL") or "gemini-2.0-flash"

    def judge(
        self,
        *,
        rubric: str,
        run: Dict[str, Any],
        threshold: float = 0.7,
        few_shot: str = "",
        model: Optional[str] = None,
    ) -> JudgeResult:
        model_id = model or self.model
        prompt = JUDGE_SYSTEM + "\n\n" + build_user_prompt(
            rubric, run, threshold=threshold, few_shot=few_shot
        )
        resp = self._client.models.generate_content(model=model_id, contents=prompt)
        content = getattr(resp, "text", None) or str(resp)
        return parse_judge_content(content, model_id, threshold)


class OllamaJudge:
    """Wrapper around the official ``ollama`` Python SDK (local models)."""

    name = "ollama"

    def __init__(self, model: Optional[str] = None, host: Optional[str] = None):
        try:
            import ollama
        except ImportError as exc:
            raise ImportError("Install ollama: pip install 'gust-sdk[judge-ollama]'") from exc
        self._mod = ollama
        kwargs: Dict[str, Any] = {}
        if host:
            kwargs["host"] = host
        elif os.getenv("OLLAMA_HOST"):
            kwargs["host"] = os.environ["OLLAMA_HOST"]
        self._client = ollama.Client(**kwargs) if kwargs else ollama.Client()
        self.model = model or os.getenv("GUST_JUDGE_MODEL") or "llama3.1:8b"

    def judge(
        self,
        *,
        rubric: str,
        run: Dict[str, Any],
        threshold: float = 0.7,
        few_shot: str = "",
        model: Optional[str] = None,
    ) -> JudgeResult:
        model_id = model or self.model
        resp = self._client.chat(
            model=model_id,
            format="json",
            messages=[
                {"role": "system", "content": JUDGE_SYSTEM},
                {
                    "role": "user",
                    "content": build_user_prompt(rubric, run, threshold=threshold, few_shot=few_shot),
                },
            ],
        )
        content = (resp.get("message") or {}).get("content") or ""
        return parse_judge_content(content, model_id, threshold)


class GenericJudge:
    """OpenAI-compatible custom endpoint via the official ``openai`` SDK ``base_url``.

    Use this when your provider is not OpenAI/Anthropic/Google/Ollama but speaks
    the chat-completions API (vLLM, Azure OpenAI, LiteLLM proxy, etc.).
    """

    name = "generic"

    def __init__(
        self,
        model: Optional[str] = None,
        api_key: Optional[str] = None,
        base_url: Optional[str] = None,
    ):
        base = base_url or os.getenv("GUST_JUDGE_ENDPOINT") or os.getenv("OPENAI_BASE_URL")
        if not base:
            raise ValueError("generic judge requires base_url or GUST_JUDGE_ENDPOINT / OPENAI_BASE_URL")
        self._inner = OpenAIJudge(model=model, api_key=api_key or os.getenv("GUST_JUDGE_API_KEY"), base_url=base)
        self.model = self._inner.model

    def judge(self, **kwargs: Any) -> JudgeResult:
        return self._inner.judge(**kwargs)


def create_judge(provider: Optional[str] = None, **kwargs: Any) -> JudgeClient:
    """Factory for official-SDK judges (plus generic OpenAI-compatible)."""
    name = (provider or os.getenv("GUST_JUDGE_PROVIDER") or "openai").strip().lower()
    if name in ("openai",):
        return OpenAIJudge(**kwargs)
    if name in ("anthropic", "claude"):
        return AnthropicJudge(**kwargs)
    if name in ("google", "gemini"):
        return GoogleJudge(**kwargs)
    if name in ("ollama",):
        return OllamaJudge(**kwargs)
    if name in ("generic", "custom", "openai_compatible"):
        return GenericJudge(**kwargs)
    raise ValueError(
        f"unknown judge provider {name!r}; "
        "want openai|anthropic|google|ollama|generic"
    )


class LLMJudgePlugin:
    """Wire evaluator named ``llm_judge`` backed by :func:`create_judge`.

    Subclass or construct with an explicit client; default reads env.
    """

    # Used when composed into EvaluatorPlugin via examples/llm_judge_plugin.py
    name = "llm_judge"
    version = "1.0.0"
    description = "Optional LLM judge using official provider SDKs"
    capabilities = ["llm_judge", "soft"]

    def __init__(self, client: Optional[JudgeClient] = None):
        self._client = client

    def _client_or_create(self) -> JudgeClient:
        if self._client is not None:
            return self._client
        return create_judge()

    def evaluate(
        self,
        run: Dict[str, Any],
        expected: Optional[Dict[str, Any]],
        context: Dict[str, Any],
    ) -> Dict[str, Any]:
        cfg = (context or {}).get("config") or {}
        if not cfg.get("allow_llm_judge"):
            return {
                "passed": False,
                "score": 0.0,
                "message": "llm_judge is disabled: set policy allow_llm_judge: true",
                "evidence": {"allow_llm_judge": False},
            }
        params = (expected or {}).get("parameters") or {}
        rubric = params.get("rubric") or params.get("prompt") or ""
        if not rubric:
            return {
                "passed": False,
                "score": 0.0,
                "message": "llm_judge assertion requires parameters.rubric",
            }
        threshold = float(params.get("threshold") or 0.7)
        few_shot = str(params.get("few_shot") or "")
        model = params.get("model")
        start = time.time_ns()
        result = self._client_or_create().judge(
            rubric=rubric,
            run=run,
            threshold=threshold,
            few_shot=few_shot,
            model=model,
        )
        return {
            "passed": result.passed,
            "score": result.score,
            "message": result.rationale
            or (
                f"llm_judge score {result.score:.2f} >= threshold {threshold:.2f}"
                if result.passed
                else f"llm_judge score {result.score:.2f} < threshold {threshold:.2f}"
            ),
            "evidence": {
                "score": result.score,
                "threshold": threshold,
                "model": result.model,
                "provider": getattr(self._client_or_create(), "name", "unknown"),
                "criticality": "soft",
                "calibrated": bool(cfg.get("llm_judge_calibrated")),
            },
            "execution_time_ns": time.time_ns() - start,
        }
