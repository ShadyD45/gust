---
title: Statistics and policy
nav_order: 5
parent: Architecture
math: true
---
# Statistics and Policy

## Wilson score interval

Given *n* independent trials and *k* passes, with normal quantile *z* (95% → 1.95996):

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

### Corrected Hypothesis H7 sample sizes

With *P_min* = 0.95 and 95% confidence, **N = 20 can never produce `PASS`** (even 20/20 has lower bound ≈ 0.84). Valid verification vectors:

| Intent | Example | Why |
|--------|---------|-----|
| `PASS` | 100/100 | Lower ≈ 0.963 ≥ 0.95 |
| `FLAKY` | 20/20 | Interval straddles 0.95 |
| `FAIL` | 17/20 or 40/100 | Upper &lt; 0.95 |

Synthetic H7 proofs must use rates and *N* that land in these buckets under the rules above—not the outdated “N=20 → PASS for p=0.99” wording in early phase drafts.

## Hard vs soft constraints

- **Hard:** zero-tolerance (forbidden tools, schema violations). Any violation fails the policy immediately (exit code 1), regardless of pass rate.
- **Soft:** probabilistic reliability thresholds evaluated via Wilson verdicts.

## `on_flaky`

| Mode | Overall when flaky present | Exit code |
|------|----------------------------|-----------|
| `warn` | Treat as pass with warnings | 0 |
| `ignore` | Discard flaky contribution | 0 |
| `fail` | Block CI | 3 |

## Regression comparator

Baseline vs candidate compares pass-rate drop and latency increase against policy caps. Tiny stochastic differences must not fail CI: require a statistically meaningful drop (e.g. two-proportion z-test or non-overlapping Wilson intervals) in addition to exceeding `max_pass_rate_drop`.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Failure / hard constraint / regression |
| 2 | Config or runtime error |
| 3 | Flaky under `on_flaky: fail` |

