package evaluators

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"gust/internal/adapters/judge"
	"gust/internal/ports"
	"gust/pkg/api"
)

// LLMJudgeEvaluator is an optional soft evaluator backed by a JudgeProvider.
// It is gated by EvaluationContext.Config["allow_llm_judge"] == true (or "true").
type LLMJudgeEvaluator struct {
	Provider ports.JudgeProvider
}

func (e *LLMJudgeEvaluator) Name() string    { return "llm_judge" }
func (e *LLMJudgeEvaluator) Version() string { return "1.0.0" }

func (e *LLMJudgeEvaluator) Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx ports.EvaluationContext) (ports.EvaluationResult, error) {
	start := time.Now()
	res := ports.EvaluationResult{
		EvaluatorName:    e.Name(),
		EvaluatorVersion: e.Version(),
	}

	if !allowLLMJudge(evalCtx) {
		res.Passed = false
		res.Score = 0
		res.Message = "llm_judge is disabled: set policy allow_llm_judge: true to enable (opt-in soft signal)"
		res.Evidence = map[string]any{"allow_llm_judge": false}
		res.ExecutionTimeNs = time.Since(start).Nanoseconds()
		return res, nil
	}

	provider := e.Provider
	if provider == nil {
		provider = resolveProviderFromCtx(evalCtx)
	}
	if provider == nil {
		res.Passed = false
		res.Score = 0
		res.Message = "llm_judge: no JudgeProvider configured"
		res.ExecutionTimeNs = time.Since(start).Nanoseconds()
		return res, nil
	}

	rubric := ""
	threshold := 0.7
	model := ""
	fewShot := ""
	if expected != nil && expected.Parameters != nil {
		if v, ok := expected.Parameters["rubric"].(string); ok {
			rubric = v
		}
		if v, ok := expected.Parameters["prompt"].(string); ok && rubric == "" {
			rubric = v
		}
		if v, ok := expected.Parameters["threshold"].(float64); ok {
			threshold = v
		} else if v, ok := expected.Parameters["threshold"].(int); ok {
			threshold = float64(v)
		} else if v, ok := expected.Parameters["threshold"].(string); ok {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				threshold = f
			}
		}
		if v, ok := expected.Parameters["model"].(string); ok {
			model = v
		}
		if v, ok := expected.Parameters["few_shot"].(string); ok {
			fewShot = v
		}
	}
	if rubric == "" {
		res.Passed = false
		res.Score = 0
		res.Message = "llm_judge assertion requires parameters.rubric (or parameters.prompt)"
		res.ExecutionTimeNs = time.Since(start).Nanoseconds()
		return res, nil
	}

	// Soft criticality unless policy marks the judge as calibrated.
	calibrated := boolFromConfig(evalCtx, "llm_judge_calibrated")
	if expected != nil && expected.Criticality == api.CriticalityHard && !calibrated {
		// Do not honor hard criticality until calibration gate is open.
		_ = expected // criticality forced soft in evidence only; scoring unchanged
	}

	jr, err := provider.Judge(ctx, ports.JudgeRequest{
		Run:        run,
		Rubric:     rubric,
		Threshold:  threshold,
		Model:      model,
		FewShot:    fewShot,
		Parameters: expectedParams(expected),
	})
	if err != nil {
		return res, fmt.Errorf("llm_judge provider %q: %w", provider.Name(), err)
	}

	res.Passed = jr.Passed
	res.Score = jr.Score
	res.Message = jr.Rationale
	if res.Message == "" {
		if jr.Passed {
			res.Message = fmt.Sprintf("llm_judge score %.2f >= threshold %.2f", jr.Score, threshold)
		} else {
			res.Message = fmt.Sprintf("llm_judge score %.2f < threshold %.2f", jr.Score, threshold)
		}
	}
	res.Evidence = map[string]any{
		"score":       jr.Score,
		"threshold":   threshold,
		"model":       jr.Model,
		"provider":    provider.Name(),
		"calibrated":  calibrated,
		"criticality": "soft",
		"rationale":   jr.Rationale,
	}
	if !calibrated {
		res.Evidence["note"] = "judge is experimental; results are soft-only until Spearman ρ ≥ 0.7"
	}
	res.ExecutionTimeNs = time.Since(start).Nanoseconds()
	return res, nil
}

func allowLLMJudge(evalCtx ports.EvaluationContext) bool {
	return boolFromConfig(evalCtx, "allow_llm_judge")
}

func boolFromConfig(evalCtx ports.EvaluationContext, key string) bool {
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

func expectedParams(expected *api.Assertion) map[string]any {
	if expected == nil {
		return nil
	}
	return expected.Parameters
}

func resolveProviderFromCtx(evalCtx ports.EvaluationContext) ports.JudgeProvider {
	if evalCtx.Config == nil {
		return nil
	}
	if p, ok := evalCtx.Config["judge_provider"].(ports.JudgeProvider); ok {
		return p
	}
	name, _ := evalCtx.Config["judge_provider_name"].(string)
	endpoint, _ := evalCtx.Config["judge_endpoint"].(string)
	model, _ := evalCtx.Config["judge_model"].(string)
	apiKey, _ := evalCtx.Config["judge_api_key"].(string)
	prov, err := judge.ResolveProvider(name, endpoint, model, apiKey)
	if err != nil {
		return nil
	}
	return prov
}

// NewLLMJudgeEvaluator constructs an llm_judge evaluator with the given provider
// (nil provider resolves from EvaluationContext at evaluate time).
func NewLLMJudgeEvaluator(provider ports.JudgeProvider) *LLMJudgeEvaluator {
	return &LLMJudgeEvaluator{Provider: provider}
}
