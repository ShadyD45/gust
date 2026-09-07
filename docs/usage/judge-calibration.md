---
title: Judge calibration
nav_order: 9
parent: Usage
---
# Calibrating an LLM judge

A judge that has never been compared to labeled traces is just another opinionated model. Calibration asks: **does this judge rank runs the way humans (or a frozen gold set) rank them?**

Gust uses Spearman’s rank correlation ρ on a versioned dataset of at least 50 cases. The gate is ρ ≥ 0.7. Below that, keep `llm_judge.calibrated: false` so results stay soft.

## Workflow

1. Start with the synthetic seed (proves the harness, not the model):

   ```bash
   ./gust judge calibrate \
     --dataset testdata/judge/calibration/v1.json \
     --provider mock \
     --out docs/usage/judge-calibration-mock.json
   ```

2. Replace labels with traces your team actually cares about (tone misses, wrong tools, hallucinated policy). Keep the JSON schema: each case has a run plus a human score.

3. Calibrate the same plugin you will run in CI:

   ```bash
   export GUST_JUDGE_PROVIDER=openai
   export GUST_JUDGE_MODEL=gpt-4o-mini
   ./gust judge calibrate \
     --dataset testdata/judge/calibration/human-v1.json \
     --judge-plugin sdk/python/examples/llm_judge_plugin.py
   ```

4. If ρ ≥ 0.7 on ≥ 50 cases, set policy:

   ```yaml
   allow_llm_judge: true
   llm_judge:
     calibrated: true
     spearman_rho: 0.81
     min_spearman: 0.7
   ```

   The CLI does not rewrite policy for you. That is intentional: promoting a judge to hard criticality is an ops decision.

5. Only then may `criticality: hard` on `llm_judge` or `judge_panel` fail samples and affect Wilson reliability.

## What this report is

The checked-in [`judge-calibration-mock.json`](judge-calibration-mock.json) is produced by the mock provider against the synthetic seed. It is **not** a production calibration. Do not copy `calibrated: true` into a default policy from this file.

## When not to promote

- The rubric changed and you have not re-labeled.
- The judge model changed.
- ρ is high but humans still disagree with the failures you actually see (correlation is not “this bug is real”).
- You need the check on every PR: even a calibrated judge is slow and non-deterministic. Prefer deterministic assertions on the hot path.
