package testrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

var _ ports.TestRunner = (*HTTPRunner)(nil)

// InvokeRequest is the JSON body gust POSTs to an HTTP agent.
type InvokeRequest struct {
	Input         string         `json:"input"`
	Context       map[string]any `json:"context,omitempty"`
	ToolEndpoint  string         `json:"tool_endpoint,omitempty"`
	SampleID      string         `json:"sample_id"`
	EvaluationID  string         `json:"evaluation_id,omitempty"`
	ScenarioID    string         `json:"scenario_id,omitempty"`
	TraceID       string         `json:"trace_id,omitempty"`
	TraceParent   string         `json:"traceparent,omitempty"`
	Baggage       string         `json:"baggage,omitempty"`
	OTelEndpoint  string         `json:"otel_endpoint,omitempty"`
	IngestURL     string         `json:"ingest_url,omitempty"`
	WorldControl  string         `json:"world_control,omitempty"`
}

// HTTPRunner POSTs one sample to a user-owned agent endpoint and collects the trace.
type HTTPRunner struct {
	URL       string
	Client    *http.Client
	Collector CollectorConfig
	OTelURL   string
	Timeout   time.Duration
}

// NewHTTPRunner constructs a runner targeting the given invoke URL.
func NewHTTPRunner(url string, collector CollectorConfig) *HTTPRunner {
	url = strings.TrimRight(url, "/")
	timeout := collector.timeout()
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &HTTPRunner{
		URL:       url,
		Client:    &http.Client{Timeout: timeout + 5*time.Second},
		Collector: collector,
		Timeout:   timeout,
	}
}

func (r *HTTPRunner) Name() string { return "http" }

func (r *HTTPRunner) WithOTelURL(url string) *HTTPRunner {
	r.OTelURL = url
	return r
}

func (r *HTTPRunner) Run(ctx context.Context, req ports.SampleRequest) (api.AgentRun, error) {
	if r.URL == "" {
		return api.AgentRun{}, fmt.Errorf("http runner: agent URL is required")
	}
	req = normalizeSampleRequest(req, r.OTelURL)

	payload, err := json.Marshal(InvokeRequest{
		Input:        req.Scenario.Task.Input,
		Context:      req.Scenario.Task.Context,
		ToolEndpoint: req.FixtureEndpointOrEmpty(),
		SampleID:     req.SampleID,
		EvaluationID: req.EvaluationID,
		ScenarioID:   req.ScenarioID,
		TraceID:      req.TraceID,
		TraceParent:  req.TraceParent,
		Baggage:      req.Baggage,
		OTelEndpoint: req.OTelEndpoint,
		IngestURL:    req.IngestURL,
		WorldControl: string(req.WorldMode),
	})
	if err != nil {
		return api.AgentRun{}, err
	}

	invokeURL := r.URL
	if !strings.Contains(strings.TrimPrefix(strings.TrimPrefix(invokeURL, "http://"), "https://"), "/") {
		invokeURL = invokeURL + "/invoke"
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, invokeURL, bytes.NewReader(payload))
	if err != nil {
		return api.AgentRun{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Gust-Sample-Id", req.SampleID)
	if req.EvaluationID != "" {
		httpReq.Header.Set("X-Gust-Evaluation-Id", req.EvaluationID)
	}
	if req.TraceParent != "" {
		httpReq.Header.Set("traceparent", req.TraceParent)
	}
	if req.Baggage != "" {
		httpReq.Header.Set("baggage", req.Baggage)
	}

	resp, err := r.Client.Do(httpReq)
	if err != nil {
		return api.AgentRun{}, classifyInvokeErr(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return api.AgentRun{}, fmt.Errorf("read agent response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return api.AgentRun{}, fmt.Errorf("invoke agent: HTTP %d: %s", resp.StatusCode, truncate(body, 512))
	}

	run, err := Collect(ctx, r.Collector, req.SampleID, body)
	if err != nil {
		return api.AgentRun{}, err
	}
	stampSampleMetadata(&run, req)
	return run, nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}

func classifyInvokeErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("invoke agent: %w", err)
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return fmt.Errorf("invoke agent: %w: %v", ports.ErrTransient, err)
	}
	return fmt.Errorf("invoke agent: %w", err)
}
