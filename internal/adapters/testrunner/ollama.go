package testrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

var _ ports.TestRunner = (*OllamaRunner)(nil)

// OllamaRunner calls a local OpenAI-compatible chat completions endpoint (Ollama).
type OllamaRunner struct {
	BaseURL    string
	Model      string
	HTTPClient *http.Client
	AgentName  string
}

// NewOllamaRunner constructs a runner targeting the given base URL (e.g. http://localhost:11434).
func NewOllamaRunner(baseURL, model string) *OllamaRunner {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "llama3.1:8b"
	}
	return &OllamaRunner{
		BaseURL:    baseURL,
		Model:      model,
		HTTPClient: &http.Client{Timeout: 120 * time.Second},
		AgentName:  "ollama-agent",
	}
}

func (r *OllamaRunner) Name() string { return "ollama" }

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Message chatMessage `json:"message"`
	Error   string      `json:"error,omitempty"`
}

func (r *OllamaRunner) Run(ctx context.Context, req ports.SampleRequest) (api.AgentRun, error) {
	fixtureEndpoint := req.FixtureEndpointOrEmpty()
	if fixtureEndpoint == "" {
		fixtureEndpoint = os.Getenv("AGENTEVAL_FIXTURE_ENDPOINT")
	}

	endpoint := r.BaseURL + "/api/chat"
	payload := chatRequest{
		Model: r.Model,
		Messages: []chatMessage{{
			Role: "user",
			Content: fmt.Sprintf(
				"You are an agent under test. Task: %s\nFixture endpoint (route tool calls here): %s\nRespond with a short completion status.",
				req.Scenario.Task.Input, fixtureEndpoint,
			),
		}},
		Stream: false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return api.AgentRun{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return api.AgentRun{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := r.HTTPClient.Do(httpReq)
	if err != nil {
		return api.AgentRun{}, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return api.AgentRun{}, err
	}
	if resp.StatusCode >= 300 {
		return api.AgentRun{}, fmt.Errorf("ollama HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return api.AgentRun{}, fmt.Errorf("decode ollama response: %w", err)
	}
	if parsed.Error != "" {
		return api.AgentRun{}, fmt.Errorf("ollama error: %s", parsed.Error)
	}

	now := time.Now().UTC()
	output := parsed.Message.Content
	status := "completed"
	if strings.TrimSpace(output) == "" {
		status = "failed"
	}

	runID := req.SampleID
	if runID == "" {
		runID = fmt.Sprintf("ollama_%s_%d", req.Scenario.ID, start.UnixNano())
	}
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         runID,
		Agent: api.AgentInfo{
			Name:    r.AgentName,
			Version: r.Model,
		},
		Task: req.Scenario.Task,
		Trace: []api.Span{{
			SpanID:    "llm_1",
			Name:      r.Model,
			Type:      api.SpanTypeLLM,
			StartTime: start.UTC(),
			EndTime:   now,
			Attributes: map[string]any{
				"output":           output,
				"fixture_endpoint": fixtureEndpoint,
			},
			Status: api.SpanStatus{Code: "ok"},
		}},
		Outcome: api.RunOutcome{
			Status:     status,
			Output:     output,
			DurationNs: time.Since(start),
		},
	}
	stampSampleMetadata(&run, req)
	return run, nil
}
