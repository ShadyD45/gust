package api

// CI exit codes used by the policy engine and CLI.
const (
	ExitSuccess      = 0 // All scenarios PASS, or FLAKY under on_flaky: warn (exit 0) / ignore (PASS)
	ExitFailure      = 1 // FAIL, hard constraint, or regression
	ExitConfigError  = 2 // Bad flags, invalid paths, unparseable input
	ExitFlakyFailure = 3 // FLAKY under on_flaky: fail
)

// FailureCategory classifies why a Mode 3 sample did not contribute a clean pass.
type FailureCategory string

const (
	FailureNone          FailureCategory = ""
	FailureTrigger       FailureCategory = "trigger"
	FailureTimeout       FailureCategory = "timeout"
	FailureFixture       FailureCategory = "fixture"
	FailureTraceIngest   FailureCategory = "trace_ingest"
	FailureEvaluator     FailureCategory = "evaluator"
	FailureAgentRuntime  FailureCategory = "agent_runtime"
	FailureAssertion     FailureCategory = "assertion"
)

// IsInfrastructure reports whether the failure prevented observation/evaluation.
// These samples are excluded from the behavioral Wilson denominator.
func (c FailureCategory) IsInfrastructure() bool {
	switch c {
	case FailureTrigger, FailureTimeout, FailureFixture, FailureTraceIngest, FailureEvaluator:
		return true
	default:
		return false
	}
}

// SampleExecutionStatus is the per-sample lifecycle status.
type SampleExecutionStatus string

const (
	SampleStatusPassed         SampleExecutionStatus = "passed"
	SampleStatusFailed         SampleExecutionStatus = "failed"
	SampleStatusExcluded       SampleExecutionStatus = "excluded"
	SampleStatusAgentFailed    SampleExecutionStatus = "agent_failed"
	SampleStatusInfraError     SampleExecutionStatus = "infrastructure_error"
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

// FixtureCallEvidence records one resolved tool call through the Gust proxy.
type FixtureCallEvidence struct {
	Tool       string         `json:"tool"`
	Arguments  map[string]any `json:"arguments,omitempty"`
	FixtureID  string         `json:"fixture_id,omitempty"`
	Mode       string         `json:"mode,omitempty"`
	Status     string         `json:"status,omitempty"`
	StatusCode int            `json:"status_code,omitempty"`
	Body       any            `json:"body,omitempty"`
	Error      string         `json:"error,omitempty"`
	Found      bool           `json:"found"`
}

// SampleResult is one Mode 3 sample outcome with correlation and evidence.
type SampleResult struct {
	SampleID         string                `json:"sample_id"`
	EvaluationID     string                `json:"evaluation_id,omitempty"`
	ScenarioID       string                `json:"scenario_id,omitempty"`
	TraceID          string                `json:"trace_id,omitempty"`
	RunID            string                `json:"run_id,omitempty"`
	Status           SampleExecutionStatus `json:"status"`
	FailureCategory  FailureCategory       `json:"failure_category,omitempty"`
	Message          string                `json:"message,omitempty"`
	Passed           bool                  `json:"passed"`
	Evaluations      []EvaluationResult    `json:"evaluations,omitempty"`
	FixtureCalls     []FixtureCallEvidence `json:"fixture_calls,omitempty"`
}

// ReliabilityResult is the Mode 3 aggregate for a scenario across repeated samples.
type ReliabilityResult struct {
	EvaluationID         string             `json:"evaluation_id,omitempty"`
	ScenarioID           string             `json:"scenario_id"`
	Samples              int                `json:"samples"` // requested; alias of SamplesRequested for compatibility
	SamplesRequested     int                `json:"samples_requested,omitempty"`
	SamplesCompleted     int                `json:"samples_completed,omitempty"`
	Passes               int                `json:"passes"`
	BehavioralFailures   int                `json:"behavioral_failures,omitempty"`
	InfrastructureErrors int                `json:"infrastructure_errors,omitempty"`
	ObservedPassRate     float64            `json:"observed_pass_rate"`
	ConfidenceInterval   [2]float64         `json:"confidence_interval"`
	Verdict              VerdictType        `json:"verdict"`
	PerRunEvidence       []EvaluationResult `json:"per_run_evidence,omitempty"`
	SampleResults        []SampleResult     `json:"sample_results,omitempty"`
	HardConstraintFailed bool               `json:"hard_constraint_failed,omitempty"`
	ExecutionErrors      int                `json:"execution_errors,omitempty"` // alias of InfrastructureErrors
}
