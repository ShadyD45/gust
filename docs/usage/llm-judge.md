---
title: LLM judge
nav_order: 7
parent: Usage
---
# Optional LLM judge

Deterministic evaluators stay the default CI path. An LLM judge is an **opt-in soft signal** for open-ended checks (tone, helpfulness, summary quality) that sit *beside* tool/schema assertions — never replace them.

## Architecture

| Layer | Role |
|-------|------|
| Go `ports.JudgeProvider` + `llm_judge` evaluator | Gating, scoring shape, Mode 1/3 integration |
| **Official Python SDKs** (`gust_sdk.judge`) | OpenAI, Anthropic, Google GenAI, Ollama wrappers |
| Go `generic` provider | Thin OpenAI-compatible HTTP escape hatch for custom endpoints without a Python plugin |
| Tier-2 `--judge-plugin` | Loads a Python plugin that **overrides** the built-in `llm_judge` |

Major clouds should use the official SDKs. Do not reimplement provider clients in Go.

## Enable

Policy (default is off — CI stays offline and bit-identical):

```yaml
version: "1.0"
name: with-judge
allow_llm_judge: true
llm_judge:
  calibrated: false   # set true only after gust judge calibrate passes ρ ≥ 0.7
reliability:
  default_minimum_pass_rate: 0.95
  min_samples_for_verdict: 5
  on_flaky: warn
```

Assertion:

```json
{
  "id": "helpful_cancel",
  "type": "llm_judge",
  "criticality": "soft",
  "parameters": {
    "rubric": "Confirm the order was cancelled in a clear, helpful tone.",
    "threshold": 0.7
  }
}
```

Until `llm_judge.calibrated: true`, results are treated as **soft / experimental** even if you write `criticality: hard`.

## Official SDK plugins (recommended)

```bash
pip install 'gust-sdk[judge]'          # openai + anthropic + google-genai + ollama
# or: pip install 'gust-sdk[judge-openai]'
```

```bash
export GUST_JUDGE_PROVIDER=openai      # openai | anthropic | google | ollama | generic
export GUST_JUDGE_MODEL=gpt-4o-mini
export OPENAI_API_KEY=...

gust analyze run.json --policy policy.yaml \
  --judge-plugin sdk/python/examples/llm_judge_plugin.py
```

| Provider | Package | Env |
|----------|---------|-----|
| OpenAI | `openai` | `OPENAI_API_KEY` |
| Anthropic | `anthropic` | `ANTHROPIC_API_KEY` |
| Google | `google-genai` | `GOOGLE_API_KEY` / `GEMINI_API_KEY` |
| Ollama | `ollama` | `OLLAMA_HOST` (optional) |
| Generic (vLLM, Azure, LiteLLM, …) | `openai` with `base_url` | `GUST_JUDGE_ENDPOINT` + `GUST_JUDGE_API_KEY` |

Judge plugins receive an allowlisted set of API-key env vars (deterministic wire evaluators still get a scrubbed environment).

## Built-in Go providers (no Python)

```bash
export GUST_JUDGE_PROVIDER=generic
export GUST_JUDGE_ENDPOINT=http://localhost:11434   # OpenAI-compatible base
export GUST_JUDGE_MODEL=llama3.1:8b

gust analyze run.json --policy policy.yaml
```

`mock` is for tests/calibration dry-runs only.

## Calibration gate

```bash
gust judge calibrate \
  --dataset testdata/judge/calibration/v1.json \
  --provider mock
```

Gate: Spearman ρ ≥ 0.7 on ≥ 50 labeled cases. Below that, keep `calibrated: false`. Prefer calibrating the same official SDK plugin you run in CI (mock proves the harness; live labels prove the model).

## Invariants

- `allow_llm_judge: false` (default) → `llm_judge` assertions fail closed with a clear message; no network.
- Disabling the judge restores bit-identical Mode 1/2 behavior for suites without `llm_judge` assertions.
- Demo / default policies never auto-enable the judge.
