package analyze

import (
	"context"
	"fmt"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

// AnalysisReport holds the aggregated results of running evaluators across an AgentRun.
type AnalysisReport struct {
	RunID           string                   `json:"run_id"`
	Passed          bool                     `json:"passed"`
	Results         []ports.EvaluationResult `json:"results"`
	TotalDurationNs int64                    `json:"total_duration_ns"`
}

// Engine implements Mode 1 (Analyze): evaluating captured runs without execution.
type Engine struct {
	evaluators map[string]ports.Evaluator
}

// NewEngine initializes the Analyze engine with registered evaluators.
func NewEngine(evaluators []ports.Evaluator) *Engine {
	m := make(map[string]ports.Evaluator)
	for _, e := range evaluators {
		m[e.Name()] = e
	}
	return &Engine{evaluators: m}
}

// AnalyzeRun evaluates the run against provided assertions.
func (e *Engine) AnalyzeRun(ctx context.Context, run api.AgentRun, assertions []api.Assertion, evalCtx ports.EvaluationContext) (*AnalysisReport, error) {
	start := time.Now()

	report := &AnalysisReport{
		RunID:   run.RunID,
		Passed:  true,
		Results: make([]ports.EvaluationResult, 0, len(assertions)),
	}

	calibrated := boolFromEvalConfig(evalCtx, "llm_judge_calibrated")
	peers := withPeerEvaluators(evalCtx, e.evaluators)

	for _, assert := range assertions {
		evalName := resolveEvaluatorName(assert)
		evaluator, ok := e.evaluators[evalName]
		if !ok {
			return nil, fmt.Errorf("evaluator %q not found for assertion %q", evalName, assert.ID)
		}

		res, err := evaluator.Evaluate(ctx, run, &assert, peers)
		if err != nil {
			return nil, fmt.Errorf("evaluation failed for assertion %q: %w", assert.ID, err)
		}

		crit := api.SampleCriticality(assert, calibrated)
		res.Criticality = crit
		if !res.Passed && api.AssertionPolicyHard(assert) && crit != api.CriticalitySoft {
			if res.Evidence == nil {
				res.Evidence = map[string]any{}
			}
			res.Evidence["policy_hard"] = true
		}
		report.Results = append(report.Results, res)
		if !res.Passed && crit != api.CriticalitySoft {
			report.Passed = false
		}
	}

	report.TotalDurationNs = time.Since(start).Nanoseconds()
	return report, nil
}

func resolveEvaluatorName(assert api.Assertion) string {
	switch assert.Type {
	case api.AssertToolCall:
		if len(assert.Arguments) > 0 {
			return "tool_arguments"
		}
		return "tool_selection"
	case api.AssertForbiddenToolCall:
		return "forbidden_tool"
	case api.AssertMaxLatency:
		return "max_latency"
	case api.AssertSchemaValid:
		return "schema_validation"
	default:
		return string(assert.Type)
	}
}

func boolFromEvalConfig(evalCtx ports.EvaluationContext, key string) bool {
	if evalCtx.Config == nil {
		return false
	}
	v, ok := evalCtx.Config[key]
	if !ok {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1" || t == "yes"
	default:
		return false
	}
}

func withPeerEvaluators(evalCtx ports.EvaluationContext, peers map[string]ports.Evaluator) ports.EvaluationContext {
	cfg := make(map[string]any, len(evalCtx.Config)+1)
	for k, v := range evalCtx.Config {
		cfg[k] = v
	}
	cfg["peer_evaluators"] = peers
	evalCtx.Config = cfg
	return evalCtx
}
