# Phase 14: Production Continuous Evaluation

## Objectives

1. Sample live production traces into candidate `TestScenario` proposals.
2. Default **PII redaction** before any persistence or export.
3. Human-in-the-loop: nothing enters a dataset without review (`reviewed_by`).

## Scope

| In | Out |
|----|-----|
| Streaming/batch sampler from ingest (Phase 10) | Fully autonomous CI without humans |
| Redaction rules (emails, tokens, phone, custom) | Differential privacy research platform |
| `gust scenario propose` workflow | Public dataset publishing (needs Phase 15/16) |

## Invariants

- Extends Hypothesis H8: proposals never auto-fill assertions from behavior.
- Redaction is on by default; opt-out requires explicit flag and audit log.

## Verification

- Synthetic PII fixtures are stripped in output scenarios/runs.
- Provenance always records source run id + extraction time + empty `reviewed_by`.

## Exit criteria

Ops can run a sampler against staging traces and produce reviewable scenario drafts with redaction.
