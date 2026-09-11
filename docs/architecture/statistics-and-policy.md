---
title: Statistics and policy
nav_order: 5
parent: Architecture
math: true
---
# Statistics and Policy

## Wilson score interval

Given *n* approximately independent Bernoulli trials and *k* passes, with normal quantile *z* (95% → 1.95996):

{: .note }
Gust does not guarantee statistical independence between Mode 3 samples. Independence is an assumption of the reliability estimate and depends on the runner, model provider, environment, shared caches/rate limits, and fixture isolation. Concurrent samples (default concurrency 4) can be correlated in practice.

$$
p_{center} = \frac{k + z^2/2}{n + z^2},\quad
p_{margin} = \frac{z}{n + z^2}\sqrt{\frac{k(n-k)}{n} + \frac{z^2}{4}}
$$

Bounds are clamped to `[0, 1]`. Corner cases: *k = 0* and *k = n* are well-defined without division by zero.

## Verdict classification (§13.3)

Given minimum pass rate *P_min* (often 0.95) and minimum samples *n_min* (default 5):

| Verdict | Rule |
|---------|------|
| `INSUFFICIENT_SAMPLES` | *n* &lt; *n_min* |
| `PASS` | *p_lower* ≥ *P_min* |
| `FAIL` | *p_upper* &lt; *P_min* |
| `FLAKY` | *p_lower* &lt; *P_min* ≤ *p_upper* |

### Sample sizes that can PASS a 95% floor

With *P_min* = 0.95 and 95% confidence, **N = 20 can never produce `PASS`** (even 20/20 has lower bound ≈ 0.84). Useful reference points:

| Intent | Example | Why |
|--------|---------|-----|
| `PASS` | 100/100 | Lower ≈ 0.963 ≥ 0.95 |
| `FLAKY` | 20/20 | Interval straddles 0.95 |
| `FAIL` | 17/20 or 40/100 | Upper &lt; 0.95 |

At an 80% floor, a perfect N=20 run can PASS (lower bound ≈ 0.839). See [Tuning the gate]({% link usage/tuning.md %}) and [sample sizing]({% link usage/modes-cookbook.md %}#sample-sizing).

## Hard vs soft constraints

- **Hard:** `hard_constraints.forbidden_tools` / `schema_violations` are **max allowed** failed evaluations of those evaluators (default `0` = any failure fails CI, exit 1, skipping `on_flaky`). Other `criticality: hard` assertions fail the **sample** and move Wilson; they do not use this clause. `criticality: soft` is recorded and does not fail the sample.
- **Soft:** probabilistic reliability thresholds evaluated via Wilson verdicts. Unset criticality on `llm_judge` / `judge_panel` is soft. Other unset types still fail the sample, but only forbidden-tool / schema-valid count toward the named hard-constraint tallies. Uncalibrated judges stay soft even when YAML says `criticality: hard`.

How to choose values: [Tuning the gate]({% link usage/tuning.md %}).

## Execution errors consume samples

Mode 3 counts reliability as:

```text
passes / total attempted samples
```

An execution error (runner crash, timeout, invalid config) **consumes a sample** and therefore lowers observed reliability. It is not dropped from the denominator. If the fraction of execution errors exceeds `reliability.max_execution_error_rate` (default 0.20; explicit `0` is zero tolerance), the run aborts as **runner-unstable** instead of producing a PASS/FAIL/FLAKY verdict.

That is conservative on purpose: hiding infrastructure failures would make an agent look more reliable than it is.

## Retry taxonomy

Mode 3 retries **runner errors**, not assertion failures.

| `retry.on` | Retries |
|------------|---------|
| `none` | Never |
| `transient` (default) | `ports.ErrTransient` and `net.Error`; not generic agent crashes; not `context.Canceled`; not `DeadlineExceeded` |
| `all` | Every runner error except `context.Canceled` |

Default is two attempts with 50ms backoff. An agent process that exits with a generic error is a failed sample, not a blip to be papered over.

## `on_flaky`

| Mode | Overall when flaky present | Exit code |
|------|----------------------------|-----------|
| `warn` | Treat as pass with warnings | 0 |
| `ignore` | Discard flaky contribution | 0 |
| `fail` | Block CI | 3 |

## Regression comparator

`gust compare` runs a pooled two-proportion z-test on baseline vs candidate pass counts. A pass-rate drop fails CI only when **both**:

1. The absolute drop exceeds `policy.regression.max_pass_rate_drop` (default 0.02), and
2. The difference is statistically significant at *p* &lt; 0.05.

Latency increases are gated separately by `max_latency_increase_ratio` when baseline latency is set. Tiny stochastic differences that exceed the drop cap but are not significant do **not** fail CI.

### Stats v2 diagnostics (magnitude)

Alongside the z-test gate, compare reports:

- **Cohen's d** for two proportions (pooled Bernoulli SD) and a negligible/small/medium/large label
- A **seedable bootstrap CI** on the pass-rate difference (`--bootstrap`, `--seed`)

These do **not** replace Wilson for single-scenario Mode 3 verdicts. Wilson remains the default fast path for PASS/FAIL/FLAKY.

### Sample-size recommendations

`gust recommend-samples --min-pass-rate 0.95 --confidence 0.95 --expected-rate 1.0` returns the smallest *N* such that a perfect (or expected-rate) run can attain Wilson PASS — avoiding unattainable policies like N=20 at a 95% floor.

Terminal output reports baseline/candidate rates, delta, p-value, significance, effect size, bootstrap Δ CI, policy threshold, and verdict.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Failure / hard constraint / regression |
| 2 | Config or runtime error |
| 3 | Flaky under `on_flaky: fail` |

