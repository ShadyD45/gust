package gust

import (
	"context"

	"gust/pkg/api"
)

// EvaluationContext carries optional scenario metadata into an evaluator.
type EvaluationContext struct {
	ScenarioID  string
	Environment map[string]any
	Config      map[string]any
}

// Evaluator is the public contract for a behavioral assertion check.
//
// Name() is how assertion type strings resolve to implementations. Prefer a
// stable snake_case name (for example "pii_leak").
type Evaluator interface {
	Name() string
	Version() string
	Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx EvaluationContext) (api.EvaluationResult, error)
}

// AnalysisReport is the result of Mode 1 (Analyze) against a captured run.
type AnalysisReport struct {
	RunID           string                `json:"run_id"`
	Passed          bool                  `json:"passed"`
	Results         []api.EvaluationResult `json:"results"`
	TotalDurationNs int64                 `json:"total_duration_ns"`
}
