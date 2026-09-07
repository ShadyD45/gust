package api

// CI exit codes used by the policy engine and CLI.
const (
	ExitSuccess      = 0 // All scenarios PASS, or FLAKY under on_flaky: warn (exit 0) / ignore (PASS)
	ExitFailure      = 1 // FAIL, hard constraint, or regression
	ExitConfigError  = 2 // Bad flags, invalid paths, unparseable input
	ExitFlakyFailure = 3 // FLAKY under on_flaky: fail
)

// EvaluationResult is the public shape of a single assertion evaluation.
type EvaluationResult struct {
	EvaluatorName    string           `json:"evaluator_name"`
	EvaluatorVersion string           `json:"evaluator_version"`
	Passed           bool             `json:"passed"`
	Score            float64          `json:"score"`
	Message          string           `json:"message,omitempty"`
	Evidence         map[string]any   `json:"evidence,omitempty"`
	ExecutionTimeNs  int64            `json:"execution_time_ns"`
	Criticality      CriticalityLevel `json:"criticality,omitempty"`
}

// ReliabilityResult is the Mode 3 aggregate for a scenario across repeated samples.
type ReliabilityResult struct {
	ScenarioID           string             `json:"scenario_id"`
	Samples              int                `json:"samples"`
	Passes               int                `json:"passes"`
	ObservedPassRate     float64            `json:"observed_pass_rate"`
	ConfidenceInterval   [2]float64         `json:"confidence_interval"`
	Verdict              VerdictType        `json:"verdict"`
	PerRunEvidence       []EvaluationResult `json:"per_run_evidence,omitempty"`
	HardConstraintFailed bool               `json:"hard_constraint_failed,omitempty"`
	ExecutionErrors      int                `json:"execution_errors,omitempty"`
}
