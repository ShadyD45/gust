---
title: LLM judge
nav_order: 7
parent: Usage
---
# LLM judges (and why they are a little weird)

Deterministic evaluators stay the default CI path. An LLM judge is an **opt-in soft signal** for the parts of a run that are annoyingly subjective: tone, helpfulness, “did this summary actually answer the question?” Those sit *beside* tool/schema assertions — they never replace them.

The [live-agent demo]({% link usage/live-agent-demo.md %}) is that default path: sequence, arguments, recovery, and a forbidden tool — no judge.

Yes: this is an LLM grading another LLM. The industry still does it, because humans do not want to read 10,000 traces. The trick is not to pretend the judge is ground truth. Treat it like a noisy sensor you calibrate, ensemble, and keep behind a safety rail.

## How judges are used in the wild

Teams usually pick one of three shapes:

| Pattern | What the judge returns | Typical use |
|---------|------------------------|-------------|
| **Pass/fail rubric** | score + boolean against a threshold | “Was the explanation complete and on-policy?” |
| **Scalar quality** | 0–1 or 1–5 | Ranking prompt variants |
| **Pairwise** | A vs B preference | Offline eval of two agent versions |

Most production setups then add:

1. **A written rubric** (not “be good”). Vague rubrics produce confident nonsense.
2. **Calibration against labeled traces** so you know whether the judge ranks the way humans do.
3. **Bias awareness** — judges favor fluent text, longer answers, and the model family they were trained with.
4. **A deterministic backbone** — forbidden tools, schemas, and argument checks still decide whether CI goes red.

Gust’s stance: the judge is a critic in the balcony. The stage crew (fixtures, assertions, Wilson sampling) still runs the show.

```text
deterministic assertions   →  "did it call the right tools?"
optional judge / panel     →  "did it sound like a decent teammate?"
Wilson + policy            →  "does that happen often enough to ship?"
```

## One judge

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
  "id": "helpful_tone",
  "type": "llm_judge",
  "criticality": "soft",
  "parameters": {
    "rubric": "Confirm the task completed in a clear, helpful tone.",
    "threshold": 0.7
  }
}
```

Until `llm_judge.calibrated: true`, results stay **soft / experimental** even if you write `criticality: hard`. A failed judge then shows up in evidence, but it does **not** fail the sample and does **not** move the Wilson pass rate.

Worked commands (mock provider, no API key):

```bash
export GUST_JUDGE_PROVIDER=mock
./gust analyze testdata/runs/golden_cancel.json \
  --assertions testdata/assertions/with_judge.json \
  --policy testdata/policies/policy-with-judge.yaml
```

## Several independent judges

You can attach more than one `llm_judge` assertion, each with its own rubric (tone vs factual completeness vs policy language). They evaluate the same `AgentRun` independently. That is useful when you want separate evidence streams rather than a single vote.

To actually run two *models*, load them as aliased plugins:

```bash
./gust analyze run.json --policy policy.yaml \
  --plugin openai_judge=sdk/python/examples/llm_judge_plugin.py \
  --plugin anthropic_judge=sdk/python/examples/llm_judge_plugin.py
```

(Each process reads `GUST_JUDGE_PROVIDER` at startup, so for two clouds you usually wrap the example plugin or set provider inside two small scripts.)

## A panel (`judge_panel`)

When you want one assertion that asks several judges and records the argument they had:

```json
{
  "id": "panel_tone",
  "type": "judge_panel",
  "criticality": "soft",
  "parameters": {
    "rubric": "Confirm the task completed in a clear, helpful tone.",
    "aggregation": "majority",
    "threshold": 0.7,
    "judges": [
      { "name": "openai_judge", "alias": "gpt" },
      { "name": "anthropic_judge", "alias": "claude" }
    ]
  }
}
```

Aggregation:

| Mode | Passes when |
|------|-------------|
| `majority` (default) | Strict majority of members pass (ties fail) |
| `all` | Every member passes |
| `any` | At least one member passes |
| `mean_score` | Mean score ≥ `threshold` |

Evidence includes each vote (pass/score/rationale/model), vote counts, mean, score spread, a `disagreement` flag, and the final decision. Disagreement is a feature: if your panel is split, do not treat the mean as destiny.

Nested panels are rejected. Unknown member names fail closed.

## How this feeds Gust results

- **Analyze:** each assertion becomes one `EvaluationResult`. Soft failures keep `report.passed = true`.
- **Test (Mode 3):** sample pass/fail uses hard assertions only. Wilson reliability is the fraction of samples that passed those hard checks. Soft judge output is in `per_run_evidence` for humans and dashboards, not the CI gate — until you calibrate and promote `criticality: hard`.
- **Cost / latency:** every judge call is another model request per sample × member. Panels multiply that. Keep them off the PR path if you care about minutes and tokens; nightly is a better home.
- **Failure behavior:** disabled (`allow_llm_judge: false`) fails closed with no network. Provider errors on a panel member count as a failed vote, not a crashed CLI.

Official SDK plugins:

```bash
pip install 'gust-sdk[judge]'
export GUST_JUDGE_PROVIDER=openai
export GUST_JUDGE_MODEL=gpt-4o-mini
export OPENAI_API_KEY=...

./gust test tests/scenarios --plugin sdk/python/examples/llm_judge_plugin.py
# --judge-plugin still works as a deprecated alias
```

| Provider | Package | Env |
|----------|---------|-----|
| OpenAI | `openai` | `OPENAI_API_KEY` |
| Anthropic | `anthropic` | `ANTHROPIC_API_KEY` |
| Google | `google-genai` | `GOOGLE_API_KEY` / `GEMINI_API_KEY` |
| Ollama | `ollama` | `OLLAMA_HOST` (optional) |
| Generic | `openai` + `base_url` | `GUST_JUDGE_ENDPOINT` + `GUST_JUDGE_API_KEY` |

Judge plugins receive an allowlisted set of API-key env vars. Ordinary `--plugin` evaluators still get a scrubbed environment.

Go built-ins without Python: `GUST_JUDGE_PROVIDER=generic` or `mock`. `mock` is for tests and calibration dry-runs only.

## Invariants

- `allow_llm_judge: false` (default) → `llm_judge` / `judge_panel` fail closed; no network.
- Uncalibrated judges cannot fail a sample, even with `criticality: hard`.
- Demo / default policies never auto-enable the judge.
- Calibration: [Judge calibration]({% link usage/judge-calibration.md %}).
