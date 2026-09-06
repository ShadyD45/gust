# Phase 12: Optional Calibrated LLM Judge

## Status

**Shipped (Wave 1).** Optional `llm_judge` evaluator with policy `allow_llm_judge` (default false). Major providers use **official Python SDKs** via `gust_sdk.judge` + `--judge-plugin`. Go ships `mock` + thin `generic` OpenAI-compatible HTTP only. Calibration: `gust judge calibrate` (Spearman ρ ≥ 0.7 on ≥ 50 cases).

## Objectives

1. Add an optional `JudgeProvider` port for subjective/open-ended checks.
2. **Hard gate:** enable in default policies only when Spearman ρ ≥ 0.7 on ≥ 50 human-labeled calibration cases.
3. Prefer local models (e.g. Llama 3 via Ollama) to keep cost near zero for development.

## Scope

| In | Out |
|----|-----|
| `ports.JudgeProvider`, local Ollama judge adapter | Replacing deterministic evaluators |
| Calibration harness + correlation report | Uncalibrated “LLM score” as CI gate |
| Policy flag: `allow_llm_judge: false` by default | Shipping judge without calibration data |

## Invariants

- Deterministic evaluators remain the default CI path.
- Judge results are always labeled `soft` criticality unless calibration proves otherwise.
- Calibration dataset is versioned and content-addressed (JCS).

## Verification

- Calibration job prints Spearman ρ and blocks enablement below 0.7.
- Disabling judge restores bit-identical Mode 1/2 behavior.

## Exit criteria

Documented calibration report checked into repo; judge usable as opt-in soft signal only.
