"""Unit tests for judge helpers that do not require provider SDKs."""

from __future__ import annotations

from gust_sdk.judge import LLMJudgePlugin, build_user_prompt, parse_judge_content


def test_parse_judge_content_json():
    r = parse_judge_content('{"score": 0.8, "passed": true, "rationale": "ok"}', "m", 0.7)
    assert r.passed and r.score == 0.8
    assert r.rationale == "ok"


def test_parse_judge_content_markdown_fence():
    raw = 'Here you go:\n```json\n{"score": 0.2, "rationale": "bad"}\n```'
    r = parse_judge_content(raw, "m", 0.7)
    assert not r.passed and r.score == 0.2


def test_build_user_prompt_includes_rubric():
    prompt = build_user_prompt(
        "be clear",
        {"task": {"input": "cancel"}, "outcome": {"status": "completed", "output": "done"}, "trace": []},
    )
    assert "be clear" in prompt
    assert "cancel" in prompt


def test_plugin_disabled_without_allow_flag():
    plugin = LLMJudgePlugin(client=_FakeClient())
    out = plugin.evaluate(
        {"outcome": {"status": "completed", "output": "x"}, "task": {"input": "t"}, "trace": []},
        {"parameters": {"rubric": "ok"}},
        {"config": {}},
    )
    assert out["passed"] is False
    assert out["evidence"]["allow_llm_judge"] is False


def test_plugin_uses_client_when_allowed():
    plugin = LLMJudgePlugin(client=_FakeClient(score=0.95))
    out = plugin.evaluate(
        {"outcome": {"status": "completed", "output": "x"}, "task": {"input": "t"}, "trace": []},
        {"parameters": {"rubric": "ok", "threshold": 0.7}},
        {"config": {"allow_llm_judge": True}},
    )
    assert out["passed"] is True
    assert out["score"] == 0.95


class _FakeClient:
    name = "fake"

    def __init__(self, score: float = 0.9):
        self.score = score

    def judge(self, **kwargs):
        from gust_sdk.judge import JudgeResult

        thr = kwargs.get("threshold", 0.7)
        return JudgeResult(score=self.score, passed=self.score >= thr, rationale="fake", model="fake")
