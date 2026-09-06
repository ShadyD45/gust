package cli

import (
	"os"

	"gust/internal/adapters/judge"
	"gust/internal/ports"
	"gust/pkg/api"
)

// evalContextFromPolicy builds an EvaluationContext that carries judge gates and provider hints.
func evalContextFromPolicy(pol api.Policy, scenarioID string) ports.EvaluationContext {
	cfg := map[string]any{
		"allow_llm_judge":      pol.AllowLLMJudge,
		"llm_judge_calibrated": pol.LLMJudge.Calibrated,
	}
	if pol.AllowLLMJudge {
		name := os.Getenv("GUST_JUDGE_PROVIDER")
		if name == "" {
			name = "generic"
		}
		endpoint := os.Getenv("GUST_JUDGE_ENDPOINT")
		model := os.Getenv("GUST_JUDGE_MODEL")
		apiKey := os.Getenv("GUST_JUDGE_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		cfg["judge_provider_name"] = name
		cfg["judge_endpoint"] = endpoint
		cfg["judge_model"] = model
		cfg["judge_api_key"] = apiKey
		// Only resolve built-in Go providers (mock|generic). Official SDKs load via --judge-plugin.
		if name == "mock" || name == "generic" || name == "openai_compatible" || name == "http" || name == "custom" || name == "" {
			if prov, err := judge.ResolveProvider(name, endpoint, model, apiKey); err == nil {
				cfg["judge_provider"] = prov
			}
		}
	}
	return ports.EvaluationContext{
		ScenarioID: scenarioID,
		Config:     cfg,
	}
}
