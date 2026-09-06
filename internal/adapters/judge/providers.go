package judge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"gust/internal/ports"
)

var _ ports.JudgeProvider = (*GenericProvider)(nil)
var _ ports.JudgeProvider = (*MockProvider)(nil)

const defaultJudgeSystem = `You are an evaluation judge for an autonomous agent.
Score how well the agent satisfied the rubric on a scale from 0.0 to 1.0.
Respond with ONLY a single JSON object (no markdown):
{"score": <number 0-1>, "passed": <bool>, "rationale": "<short reason>"}`

// GenericProvider calls any OpenAI-compatible chat-completions endpoint.
// Use this for custom / self-hosted models. Major cloud providers should use
// the official Python SDK wrappers (gust_sdk.judge) via a Tier-2 wire plugin.
type GenericProvider struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

// NewGenericProvider constructs a BYO OpenAI-compatible judge.
// baseURL examples: https://api.openai.com, http://localhost:11434 (Ollama /v1), a vLLM URL.
func NewGenericProvider(baseURL, apiKey, model string) *GenericProvider {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if apiKey == "" {
		apiKey = firstEnv("GUST_JUDGE_API_KEY", "OPENAI_API_KEY")
	}
	if model == "" {
		model = "llama3.1:8b"
	}
	return &GenericProvider{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		Model:      model,
		HTTPClient: &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *GenericProvider) Name() string { return "generic" }

func (p *GenericProvider) Judge(ctx context.Context, req ports.JudgeRequest) (ports.JudgeResult, error) {
	model := req.Model
	if model == "" {
		model = p.Model
	}
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": defaultJudgeSystem},
			{"role": "user", "content": BuildUserPrompt(req)},
		},
		"temperature": 0,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ports.JudgeResult{}, err
	}

	// Prefer /v1/chat/completions; fall back to Ollama-native /api/chat if needed.
	endpoint := p.BaseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ports.JudgeResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	resp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return ports.JudgeResult{}, fmt.Errorf("generic judge request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return ports.JudgeResult{}, err
	}
	if resp.StatusCode >= 300 {
		// Retry once against Ollama-native path for local hosts.
		if strings.Contains(p.BaseURL, "11434") || strings.Contains(string(raw), "Not Found") {
			return p.judgeOllamaNative(ctx, req, model)
		}
		return ports.JudgeResult{}, fmt.Errorf("generic judge HTTP %d: %s", resp.StatusCode, string(raw))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ports.JudgeResult{}, fmt.Errorf("decode generic judge response: %w", err)
	}
	if parsed.Error != nil {
		return ports.JudgeResult{}, fmt.Errorf("generic judge error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return ports.JudgeResult{}, fmt.Errorf("generic judge returned no choices")
	}
	return ParseJudgeContent(parsed.Choices[0].Message.Content, model, req.Threshold)
}

func (p *GenericProvider) judgeOllamaNative(ctx context.Context, req ports.JudgeRequest, model string) (ports.JudgeResult, error) {
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": defaultJudgeSystem},
			{"role": "user", "content": BuildUserPrompt(req)},
		},
		"stream": false,
		"format": "json",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ports.JudgeResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return ports.JudgeResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return ports.JudgeResult{}, fmt.Errorf("ollama-native judge request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return ports.JudgeResult{}, err
	}
	if resp.StatusCode >= 300 {
		return ports.JudgeResult{}, fmt.Errorf("ollama-native judge HTTP %d: %s", resp.StatusCode, string(raw))
	}
	var parsed struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ports.JudgeResult{}, err
	}
	if parsed.Error != "" {
		return ports.JudgeResult{}, fmt.Errorf("ollama-native judge error: %s", parsed.Error)
	}
	return ParseJudgeContent(parsed.Message.Content, model, req.Threshold)
}

// MockProvider is an offline judge for tests and calibration dry-runs.
type MockProvider struct {
	ScoreFn func(req ports.JudgeRequest) ports.JudgeResult
}

func (p *MockProvider) Name() string { return "mock" }

func (p *MockProvider) Judge(_ context.Context, req ports.JudgeRequest) (ports.JudgeResult, error) {
	if p.ScoreFn != nil {
		return p.ScoreFn(req), nil
	}
	threshold := req.Threshold
	if threshold <= 0 {
		threshold = 0.7
	}
	score := 0.5
	out := req.Run.Outcome.Output
	switch {
	case req.Run.Outcome.Status != "completed":
		score = 0.1
	case len(out) > 60:
		score = 0.95
	case len(out) > 40:
		score = 0.8
	case len(out) > 15:
		score = 0.55
	default:
		score = 0.35
	}
	if strings.Contains(strings.ToLower(req.Rubric), "must fail") {
		score = 0.1
	}
	return ports.JudgeResult{
		Score:     score,
		Passed:    score >= threshold,
		Rationale: "mock judge",
		Model:     "mock",
	}, nil
}

// BuildUserPrompt formats the rubric + run for a judge model.
func BuildUserPrompt(req ports.JudgeRequest) string {
	var b strings.Builder
	b.WriteString("Rubric:\n")
	b.WriteString(req.Rubric)
	b.WriteString("\n\n")
	if req.FewShot != "" {
		b.WriteString("Examples:\n")
		b.WriteString(req.FewShot)
		b.WriteString("\n\n")
	}
	b.WriteString("Task input:\n")
	b.WriteString(req.Run.Task.Input)
	b.WriteString("\n\nAgent outcome status: ")
	b.WriteString(req.Run.Outcome.Status)
	b.WriteString("\nAgent output:\n")
	b.WriteString(req.Run.Outcome.Output)
	b.WriteString("\n\nTrace summary:\n")
	for _, sp := range req.Run.Trace {
		fmt.Fprintf(&b, "- [%s] %s (%s)\n", sp.Type, sp.Name, sp.Status.Code)
	}
	threshold := req.Threshold
	if threshold <= 0 {
		threshold = 0.7
	}
	fmt.Fprintf(&b, "\nPass threshold: %.2f\n", threshold)
	return b.String()
}

// ParseJudgeContent extracts score/passed/rationale from model output.
func ParseJudgeContent(content, model string, threshold float64) (ports.JudgeResult, error) {
	if threshold <= 0 {
		threshold = 0.7
	}
	content = strings.TrimSpace(content)
	raw := content
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		content = content[start : end+1]
	}
	var parsed struct {
		Score     float64 `json:"score"`
		Passed    *bool   `json:"passed"`
		Rationale string  `json:"rationale"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		if f, err2 := strconv.ParseFloat(strings.TrimSpace(raw), 64); err2 == nil {
			parsed.Score = f
		} else {
			return ports.JudgeResult{}, fmt.Errorf("parse judge JSON: %w (raw=%q)", err, raw)
		}
	}
	if parsed.Score < 0 {
		parsed.Score = 0
	}
	if parsed.Score > 1 {
		parsed.Score = 1
	}
	passed := parsed.Score >= threshold
	if parsed.Passed != nil {
		passed = *parsed.Passed
	}
	return ports.JudgeResult{
		Score:     parsed.Score,
		Passed:    passed,
		Rationale: parsed.Rationale,
		Model:     model,
		Raw:       raw,
	}, nil
}

// ResolveProvider builds a built-in Go JudgeProvider.
// Major cloud providers (OpenAI, Anthropic, Google, Ollama SDK) live in the
// Python package gust_sdk.judge and are loaded as wire plugins named llm_judge.
func ResolveProvider(name, endpoint, model, apiKey string) (ports.JudgeProvider, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "generic", "openai_compatible", "http", "custom":
		return NewGenericProvider(endpoint, apiKey, model), nil
	case "mock":
		return &MockProvider{}, nil
	case "openai", "anthropic", "google", "gemini", "ollama":
		return nil, fmt.Errorf(
			"provider %q uses the official Python SDK — install gust-sdk[judge] and load "+
				"sdk/python/examples/llm_judge_plugin.py via --judge-plugin (or assert with a wire evaluator). "+
				"For a custom OpenAI-compatible endpoint use --provider generic",
			name,
		)
	default:
		return nil, fmt.Errorf("unknown built-in judge provider %q (want mock|generic)", name)
	}
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}
