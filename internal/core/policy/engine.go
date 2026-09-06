package policy

import (
	"fmt"
	"strings"

	"gust/pkg/api"
)

// PolicyViolation records a single policy clause failure or warning.
type PolicyViolation struct {
	ClauseType string `json:"clause_type"` // hard_constraint | reliability | regression
	Message    string `json:"message"`
	Severity   string `json:"severity"` // fatal | warning
}

// Verdict is the aggregated CI decision for a set of scenario results.
type Verdict struct {
	OverallVerdict  api.VerdictType               `json:"overall_verdict"`
	ExitCode        int                           `json:"exit_code"`
	Violations      []PolicyViolation             `json:"violations"`
	ScenarioResults map[string]*api.ReliabilityResult `json:"scenario_results"`
}

// Engine evaluates Policy against Mode 3 reliability results.
type Engine struct{}

// NewEngine creates a policy engine.
func NewEngine() *Engine { return &Engine{} }

// Evaluate applies hard constraints, reliability verdicts, and on_flaky handling.
func (e *Engine) Evaluate(policy api.Policy, results []*api.ReliabilityResult) Verdict {
	verdict := Verdict{
		OverallVerdict:  api.VerdictPass,
		ExitCode:        api.ExitSuccess,
		ScenarioResults: make(map[string]*api.ReliabilityResult),
	}

	hasFlaky := false

	for _, res := range results {
		if res == nil {
			continue
		}
		verdict.ScenarioResults[res.ScenarioID] = res

		if res.HardConstraintFailed || hasHardEvidence(res) {
			verdict.OverallVerdict = api.VerdictFail
			verdict.ExitCode = api.ExitFailure
			verdict.Violations = append(verdict.Violations, PolicyViolation{
				ClauseType: "hard_constraint",
				Severity:   "fatal",
				Message:    fmt.Sprintf("%s violated a hard constraint (forbidden tool or schema)", res.ScenarioID),
			})
			return verdict
		}

		switch res.Verdict {
		case api.VerdictFail, api.VerdictInsufficientSamples:
			verdict.OverallVerdict = api.VerdictFail
			verdict.ExitCode = api.ExitFailure
			verdict.Violations = append(verdict.Violations, PolicyViolation{
				ClauseType: "reliability",
				Severity:   "fatal",
				Message:    fmt.Sprintf("%s reliability verdict %s (pass rate %.3f)", res.ScenarioID, res.Verdict, res.ObservedPassRate),
			})
		case api.VerdictFlaky:
			hasFlaky = true
			verdict.Violations = append(verdict.Violations, PolicyViolation{
				ClauseType: "reliability",
				Severity:   "warning",
				Message:    fmt.Sprintf("%s pass rate is inconclusive (FLAKY)", res.ScenarioID),
			})
		}
	}

	if hasFlaky && verdict.OverallVerdict != api.VerdictFail {
		switch strings.ToLower(policy.Reliability.OnFlaky) {
		case "fail":
			verdict.OverallVerdict = api.VerdictFlaky
			verdict.ExitCode = api.ExitFlakyFailure
		case "warn", "ignore", "":
			verdict.OverallVerdict = api.VerdictPass
			verdict.ExitCode = api.ExitSuccess
		}
	}

	return verdict
}

func hasHardEvidence(res *api.ReliabilityResult) bool {
	for _, ev := range res.PerRunEvidence {
		if ev.Passed {
			continue
		}
		if ev.EvaluatorName == "forbidden_tool" || ev.EvaluatorName == "schema_validation" {
			return true
		}
	}
	return false
}
