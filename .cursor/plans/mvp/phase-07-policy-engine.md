# Phase 7: Policy Engine & Regression Comparator

## 1. Objectives & Scope
1. Implement the **Policy Engine** decoupling raw evaluation metrics from the final CI pass/fail authority.
2. Support **Hard Constraints** (zero-tolerance violations such as forbidden tools or schema failures) alongside **Soft Probabilistic Thresholds** (pass rates with Wilson score intervals).
3. Implement configurable handling of `FLAKY` results:
   - `warn`: Emits visible warning annotation, passes CI (exit code 0).
   - `fail`: Blocks CI (exit code 3).
   - `ignore`: Disregards scenario outcome.
4. Implement the **Baseline vs. Candidate Regression Comparator** to detect statistically significant regressions across multiple dataset cases.

---

## 2. Package Architecture

```text
gust/
├── internal/
│   └── core/
│       └── policy/
│           ├── engine.go        # Policy evaluator & verdict generator
│           ├── rules.go         # Constraint checks (hard vs soft)
│           ├── regression.go    # Baseline-vs-candidate comparator
│           └── policy_test.go
└── pkg/
    └── api/
        ├── policy.go            # Policy definition schema
        └── verdict.go           # Verdict types and exit code constants
```

---

## 3. Policy Specification & Engine Logic

```yaml
policy:
  version: "1.0"
  name: "production-release-gate"

  hard_constraints:
    forbidden_tools: 0
    schema_violations: 0

  reliability:
    default_minimum_pass_rate: 0.95
    min_samples_for_verdict: 5
    on_flaky: warn # warn | fail | ignore

  regression:
    max_pass_rate_drop: 0.02
    max_latency_increase_ratio: 0.15
```

### 3.1 Verdict Generation Algorithm
```go
package policy

import (
    "gust/internal/core/test"
    "gust/pkg/api"
)

type PolicyVerdict struct {
    OverallVerdict api.VerdictType              `json:"overall_verdict"` // PASS | FAIL | FLAKY
    ExitCode       int                          `json:"exit_code"`
    Violations     []PolicyViolation            `json:"violations"`
    ScenarioResults map[string]*test.ScenarioResult `json:"scenario_results"`
}

type PolicyViolation struct {
    ClauseType string `json:"clause_type"` // "hard_constraint", "reliability", "regression"
    Message    string `json:"message"`
    Severity   string `json:"severity"`    // "fatal", "warning"
}

func (e *Engine) Evaluate(policy api.Policy, results []*test.ScenarioResult, regression *api.RegressionComparison) PolicyVerdict {
    verdict := PolicyVerdict{
        OverallVerdict: api.VerdictPass,
        ExitCode:       0,
        ScenarioResults: make(map[string]*test.ScenarioResult),
    }

    hasFlaky := false

    for _, res := range results {
        verdict.ScenarioResults[res.ScenarioID] = res

        // 1. Check Hard Constraints across per-run evidence
        for _, runEval := range res.PerRunEvidence {
            if runEval.HasForbiddenToolViolation {
                verdict.OverallVerdict = api.VerdictFail
                verdict.ExitCode = 1
                verdict.Violations = append(verdict.Violations, PolicyViolation{
                    ClauseType: "hard_constraint",
                    Severity:   "fatal",
                    Message:    "Forbidden tool called during execution",
                })
                return verdict
            }
        }

        // 2. Evaluate Reliability Verdict
        switch res.Verdict {
        case api.VerdictFail:
            verdict.OverallVerdict = api.VerdictFail
            verdict.ExitCode = 1
            verdict.Violations = append(verdict.Violations, PolicyViolation{
                ClauseType: "reliability",
                Severity:   "fatal",
                Message:    res.ScenarioID + " pass rate fell below minimum required threshold",
            })
        case api.VerdictFlaky:
            hasFlaky = true
            verdict.Violations = append(verdict.Violations, PolicyViolation{
                ClauseType: "reliability",
                Severity:   "warning",
                Message:    res.ScenarioID + " pass rate is inconclusive (FLAKY)",
            })
        }
    }

    // 3. Resolve Flaky Policy Behavior
    if hasFlaky && verdict.OverallVerdict != api.VerdictFail {
        switch policy.Reliability.OnFlaky {
        case "fail":
            verdict.OverallVerdict = api.VerdictFlaky
            verdict.ExitCode = 3
        case "warn":
            verdict.OverallVerdict = api.VerdictPass // Pass with warnings
            verdict.ExitCode = 0
        case "ignore":
            verdict.OverallVerdict = api.VerdictPass
            verdict.ExitCode = 0
        }
    }

    return verdict
}
```

---

## 4. Baseline vs. Candidate Regression Comparator

Evaluates between-experiment regression across dataset cases:
1. Calculates $\Delta \text{PassRate} = \text{Rate}_{\text{candidate}} - \text{Rate}_{\text{baseline}}$.
2. Performs a two-proportion z-test (or Wilson interval overlap test) to determine if the drop is statistically significant at the 95% confidence level.
3. If $\Delta \text{PassRate} < -\text{max\_pass\_rate\_drop}$ and $p < 0.05$, flags a `REGRESSION` failure.
4. Prevents failing CI on minor stochastic fluctuations (e.g. 95.0% vs 94.8% on small sample sizes).

---

## 5. Verification & Test Plan

- **`on_flaky` Mode Divergence Test:**
  - Provide identical test results containing a `FLAKY` scenario.
  - Evaluate against `on_flaky: warn`: Assert exit code `0`, verdict `PASS`.
  - Evaluate against `on_flaky: fail`: Assert exit code `3`, verdict `FLAKY`.
- **Hard Constraint Priority Test:**
  - Provide results with 19/20 passes (95% rate), but the single failed run invoked a forbidden tool.
  - Assert policy immediately returns `FAIL` (exit code `1`) due to hard constraint violation.
- **Statistical Significance Regression Test:**
  - Compare baseline (95.0%) vs candidate (94.9%): Assert result is `PASS` (not significant).
  - Compare baseline (95.0%) vs candidate (75.0%): Assert result is `FAIL` (statistically significant regression).
