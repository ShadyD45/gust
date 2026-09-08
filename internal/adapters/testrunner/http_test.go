package testrunner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

func TestHTTPRunner_ResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/invoke" {
			t.Errorf("path %s", r.URL.Path)
		}
		var req InvokeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
		}
		if req.Input != "Cancel my latest order" {
			t.Errorf("input %q", req.Input)
		}
		if req.ToolEndpoint == "" || req.SampleID == "" {
			t.Errorf("missing fixture or sample: %+v", req)
		}
		run := validRun(req.SampleID)
		run.Task.Input = req.Input
		_ = json.NewEncoder(w).Encode(run)
	}))
	defer srv.Close()

	r := NewHTTPRunner(srv.URL, CollectorConfig{Source: TraceSourceResponse, WaitTimeout: time.Second})
	sc := api.TestScenario{
		ID:   "cancel",
		Task: api.TaskInfo{ID: "t", Input: "Cancel my latest order"},
	}
	run, err := r.Run(context.Background(), ports.SampleRequest{
		Scenario:        sc,
		FixtureEndpoint: "http://fixtures",
		WorldMode:       api.WorldControlGust,
	})
	if err != nil {
		t.Fatal(err)
	}
	if run.Task.Input != "Cancel my latest order" {
		t.Fatalf("task input %s", run.Task.Input)
	}
	if run.Metadata["sample_id"] == nil {
		t.Fatal("expected sample_id metadata")
	}
}

func TestHTTPRunner_InvalidRunIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"schema_version":"0.5"}`))
	}))
	defer srv.Close()

	r := NewHTTPRunner(srv.URL, CollectorConfig{Source: TraceSourceResponse})
	_, err := r.Run(context.Background(), ports.SampleRequest{
		Scenario: api.TestScenario{
			ID:   "x",
			Task: api.TaskInfo{ID: "t", Input: "in"},
		},
	})
	if err == nil {
		t.Fatal("expected invalid run error")
	}
}
