---
title: Tuning the gate
nav_order: 3.5
parent: Usage
---
# Tuning the gate

gust is meant to be adjusted to the job: a first CI hook, a production merge gate, or a zero-tolerance check on a named evaluator. This page is the catalog of those knobs. Recipes per command live in the [modes cookbook]({% link usage/modes-cookbook.md %}). Worked YAML for common settings is in [Examples](#examples) at the bottom.

Defaults merge **CLI flags > `policy.yaml` > `gust.yaml` > code defaults**. `gust init` writes both YAML files. `gust.yaml` is loaded from the current directory, then parents.

## Pick a profile

| Goal | Typical knobs |
|---|---|
| Adopt without blocking merges | `on_flaky: warn`, N=20, `minimum_pass_rate: 0.80` (a perfect 20/20 can `PASS`) |
| Production merge gate | `on_flaky: fail`, N≥73 at a 95% floor **or** keep an 80% floor at N=20 |
| Zero-tolerance named constraints | `hard_constraints.forbidden_tools: 0` and/or `schema_violations: 0` |
| Record a check without failing the sample | `criticality: soft` on that assertion |

Wilson sample sizing is in the [cookbook]({% link usage/modes-cookbook.md %}#sample-sizing). At a 95% floor, **no N below ~73 can ever `PASS`**, even with zero failures.

## Two different gates

A failed assertion can do one of three things:

| | Sample / Wilson | CI `hard_constraints` clause |
|---|---|---|
| `criticality: soft` | Evidence and score only | Never |
| `criticality: hard` (default except judges) | Fails the sample; lowers the pass rate | Only if the evaluator is a **named** bucket below |
| Named hard constraint exceeded | Also fails the sample if the assertion was hard | Exit **1** immediately, skips `on_flaky` |

`hard_constraints` integers are **max allowed failed evaluations** of those evaluators, not booleans.

```yaml
hard_constraints:
  forbidden_tools: 0     # failed forbidden_tool evaluations > 0 → exit 1
  schema_violations: 0   # failed schema_validation evaluations > 0 → exit 1
```

`forbidden_tools: 1` allows one miss across the scenario’s evidence and still lets Wilson / `on_flaky` decide. A failed `tool_sequence` with `criticality: hard` fails the **sample**; it does not use this clause unless you later add a named bucket for it.

## Assertions

| Knob | Where | Default | Effect |
|---|---|---|---|
| `criticality` | assertion | `hard` (judges: `soft`) | `hard` fails the sample; `soft` records `passed` / `score` only |
| `type` | assertion | required | Built-in evaluator or a `--plugin` name |

Uncalibrated `llm_judge` / `judge_panel` stay **soft** even if YAML says `criticality: hard`. Promote them only after [judge calibration]({% link usage/judge-calibration.md %}).

## Policy (`policy.yaml`)

| Knob | Default | Effect |
|---|---|---|
| `hard_constraints.forbidden_tools` | `0` | Max failed `forbidden_tool` evaluations |
| `hard_constraints.schema_violations` | `0` | Max failed `schema_validation` evaluations |
| `reliability.default_minimum_pass_rate` | `0.95` | Used when a scenario omits `minimum_pass_rate` |
| `reliability.min_samples_for_verdict` | `5` | Below this → `INSUFFICIENT_SAMPLES` |
| `reliability.on_flaky` | `warn` | `warn` exit 0, `ignore` treat as PASS, `fail` exit 3 |
| `reliability.max_execution_error_rate` | `0.20` if omitted | Fraction of runner errors before abort as runner-unstable; explicit `0` is zero tolerance |
| `reliability.retry.on` | `transient` | Which **runner** errors retry (`none` / `transient` / `all`) |
| `reliability.retry.max_attempts` | `2` | Total `Run` tries including the first |
| `reliability.retry.backoff_ms` | `50` | Wait before a retry |
| `allow_llm_judge` | `false` | Opt-in LLM judges (offline CI stays deterministic) |
| `llm_judge.calibrated` | `false` | After Spearman ρ gate; see [LLM judge]({% link usage/llm-judge.md %}) |
| `regression.max_pass_rate_drop` | unset | `gust compare` fails when the drop exceeds this **and** p < 0.05 |
| `regression.max_latency_increase_ratio` | unset | Latency regression on its own; unmeasurable baseline latency is omitted |

Retry applies to **runner errors**, not assertion failures. Agent process crashes are not retried unless `retry.on: all`. `context.Canceled` is never retried.

Scenario-level `reliability.samples`, `minimum_pass_rate`, and `confidence` override the policy defaults for that case.

## Project config (`gust.yaml`)

| Knob | Default | Effect |
|---|---|---|
| `policy` | | Path to `policy.yaml` when `--policy` is omitted |
| `test.concurrency` | `4` | Parallel samples (`1` runs in index order) |
| `test.timeout` | | Per-sample runner timeout |
| `retry.*` | same as policy | Fallback when policy omits retry |
| `wire.handshake_timeout` | `5s` | Plugin startup |
| `wire.evaluate_timeout` | `60s` | Plugin `evaluate` when the caller has no deadline |
| `plugins` | | Repeatable wire evaluators / judges |

## CLI (Mode 3)

Flags override both YAML files. Common ones: `--samples`, `--concurrency`, `--timeout`, `--policy`, `--plugin`, `--retry-on`, `--retry-max-attempts`, `--retry-backoff-ms`, `--max-execution-error-rate`. Full table: [Mode 3 in the cookbook]({% link usage/modes-cookbook.md %}#mode-3-test).

## CI exit codes

| Code | Meaning |
|---|---|
| `0` | Gate passed, or `FLAKY` under `on_flaky: warn` / `ignore` |
| `1` | Wilson `FAIL`, named hard-constraint count exceeded, or regression |
| `2` | Bad config / paths |
| `3` | `FLAKY` under `on_flaky: fail` |

Named hard-constraint breaches skip `on_flaky` and exit `1`. Details: [CI integration]({% link usage/ci-github-actions.md %}).

## Examples

These snippets only illustrate the knobs above. They are not a domain scenario.

### Adopting: visibility without blocking

```yaml
version: "1.0"
name: adopt
hard_constraints:
  forbidden_tools: 0
  schema_violations: 0
reliability:
  default_minimum_pass_rate: 0.80
  min_samples_for_verdict: 20
  on_flaky: warn
```

Pair with `reliability.samples: 20` on the scenario. A perfect 20/20 can `PASS` at an 80% floor.

### Production: inconclusive is not shippable

```yaml
reliability:
  default_minimum_pass_rate: 0.95
  min_samples_for_verdict: 5
  on_flaky: fail
```

At a 95% floor you need N≥73 for a possible `PASS`. Keep the 80% floor if you want N=20 to be able to pass.

### Zero-tolerance named constraints

```yaml
hard_constraints:
  forbidden_tools: 0
  schema_violations: 0
```

Any failed `forbidden_tool` or `schema_validation` evaluation exits `1` and skips `on_flaky`. Raise a count only if you intentionally allow that many misses.

### Soft assertion (evidence only)

```yaml
assertions:
  - id: extra_sequence
    type: tool_sequence
    criticality: soft
    parameters:
      sequence: ["lookup", "apply", "confirm"]
```

A miss is recorded in `per_run_evidence` with a score. It does not fail the sample or move Wilson.

Multi-step agents (retrieve/apply/verify, ordered polling, schema output, judges + forbidden tools): [Scenario examples]({% link usage/examples.md %}).
