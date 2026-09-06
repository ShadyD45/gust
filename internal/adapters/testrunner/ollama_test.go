package testrunner

import (
	"context"
	"net/http"
	"testing"
	"time"

	"gust/pkg/api"
)

// TestOllamaRunner_SkipIfUnavailable exercises live Ollama when present (Mode 3 DoD).
func TestOllamaRunner_SkipIfUnavailable(t *testing.T) {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get("http://127.0.0.1:11434/api/tags")
	if err != nil {
		t.Skip("ollama not running; skip live Mode 3 check")
	}
	_ = resp.Body.Close()

	r := NewOllamaRunner("http://127.0.0.1:11434", "llama3.1:8b")
	r.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	sc := api.TestScenario{
		ID:   "ollama_smoke",
		Task: api.TaskInfo{ID: "t", Input: "Reply with the single word: ok"},
		Reliability: api.ReliabilityConfig{
			Samples:         1,
			MinimumPassRate: 0.5,
			Confidence:      0.95,
		},
		Provenance: api.TestScenarioProvenance{Source: "test", ExtractedAt: time.Now().UTC()},
	}
	run, err := r.Run(context.Background(), sc, "")
	if err != nil {
		t.Fatalf("ollama run: %v", err)
	}
	if run.Outcome.Status != "completed" && run.Outcome.Status != "failed" {
		t.Fatalf("unexpected status %q", run.Outcome.Status)
	}
}
