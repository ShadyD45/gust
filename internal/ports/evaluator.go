package ports

import (
	"context"
	"gust/pkg/api"
)

// EvaluationContext carries scenario and execution metadata to the evaluator.
type EvaluationContext struct {
	ScenarioID  string         `json:"scenario_id,omitempty"`
	Environment map[string]any `json:"environment,omitempty"`
	Config      map[string]any `json:"config,omitempty"`
}

// EvaluationResult contains the result of evaluating an assertion against an AgentRun.
type EvaluationResult struct {
	EvaluatorName    string               `json:"evaluator_name"`
	EvaluatorVersion string               `json:"evaluator_version"`
	Passed           bool                 `json:"passed"`
	Score            float64              `json:"score"` // 0.0 to 1.0
	Message          string               `json:"message,omitempty"`
	Evidence         map[string]any       `json:"evidence,omitempty"`
	ExecutionTimeNs  int64                `json:"execution_time_ns"`
	Criticality      api.CriticalityLevel `json:"criticality,omitempty"`
}

// Evaluator defines the interface for evaluating an AgentRun.
type Evaluator interface {
	Name() string
	Version() string
	Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx EvaluationContext) (EvaluationResult, error)
}
