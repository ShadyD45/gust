package testrunner

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

var _ ports.TestRunner = (*HTTPRunner)(nil)

// InvokeRequest is the JSON body gust POSTs to an HTTP agent.
type InvokeRequest struct {
	Input        string         `json:"input"`
	Context      map[string]any `json:"context,omitempty"`
	ToolEndpoint string         `json:"tool_endpoint"`
	SampleID     string         `json:"sample_id"`
	OTelEndpoint string         `json:"otel_endpoint,omitempty"`
	IngestURL    string         `json:"ingest_url,omitempty"`
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

func (r *HTTPRunner) Run(ctx context.Context, scenario api.TestScenario, fixtureEndpoint string) (api.AgentRun, error) {
	if fixtureEndpoint == "" {
		fixtureEndpoint = os.Getenv(EnvFixtureEndpoint)
	}
	if r.URL == "" {
		return api.AgentRun{}, fmt.Errorf("http runner: agent URL is required")
	}

	sampleID := newSampleID(scenario.ID)
	ingestURL := ""
	if r.OTelURL != "" {
		ingestURL = strings.TrimRight(r.OTelURL, "/") + "/v1/runs"
	}
	payload, err := json.Marshal(InvokeRequest{
		Input:        scenario.Task.Input,
		Context:      scenario.Task.Context,
		ToolEndpoint: fixtureEndpoint,
		SampleID:     sampleID,
		OTelEndpoint: r.OTelURL,
		IngestURL:    ingestURL,
	})
	if err != nil {
		return api.AgentRun{}, err
	}

	invokeURL := r.URL
	if !strings.Contains(strings.TrimPrefix(strings.TrimPrefix(invokeURL, "http://"), "https://"), "/") {
		invokeURL = invokeURL + "/invoke"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, invokeURL, bytes.NewReader(payload))
	if err != nil {
		return api.AgentRun{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gust-Sample-Id", sampleID)

	resp, err := r.Client.Do(req)
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

	run, err := Collect(ctx, r.Collector, sampleID, body)
	if err != nil {
		return api.AgentRun{}, err
	}
	stampSampleMetadata(&run, sampleID)
	return run, nil
}

func newSampleID(scenarioID string) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s-%d-%s", scenarioID, time.Now().UnixNano(), hex.EncodeToString(b[:]))
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
