package testrunner

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"gust/pkg/api"
)

func TestMain(m *testing.M) {
	switch os.Getenv("GUST_EXEC_HELPER") {
	case "1":
		run := validRun(os.Getenv(EnvSampleID))
		run.Task.Input = "from-helper"
		_ = json.NewEncoder(os.Stdout).Encode(run)
		os.Exit(0)
	case "otel":
		run := validRun(os.Getenv(EnvSampleID))
		run.Metadata = map[string]any{
			"otel_endpoint": os.Getenv(EnvOTelEndpoint),
			"otel_protocol": os.Getenv(EnvOTelProtocol),
			"otel_traces":   os.Getenv(EnvOTelTracesEndpoint),
			"ingest_url":    os.Getenv(EnvIngestURL),
			"resource":      os.Getenv(EnvOTelResourceAttrs),
		}
		_ = json.NewEncoder(os.Stdout).Encode(run)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestExecRunner_Stdout(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r := NewExecRunner([]string{exe}, CollectorConfig{Source: TraceSourceResponse, WaitTimeout: 5 * time.Second})
	r.Timeout = 10 * time.Second
	os.Setenv("GUST_EXEC_HELPER", "1")
	t.Cleanup(func() { os.Unsetenv("GUST_EXEC_HELPER") })

	// The child inherits env; CommandContext also appends env. Set on parent so
	// the helper TestMain branch is taken. ExecRunner copies os.Environ().
	run, err := r.Run(context.Background(), api.TestScenario{
		ID:   "exec",
		Task: api.TaskInfo{ID: "t", Input: "do it"},
	}, "http://fixtures")
	if err != nil {
		t.Fatal(err)
	}
	if run.Task.Input != "from-helper" {
		t.Fatalf("input %s", run.Task.Input)
	}
}

func TestExecRunner_InjectsOTelEnv(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r := NewExecRunner([]string{exe}, CollectorConfig{Source: TraceSourceResponse, WaitTimeout: 5 * time.Second})
	r.Timeout = 10 * time.Second
	r.OTelURL = "http://127.0.0.1:4318"
	os.Setenv("GUST_EXEC_HELPER", "otel")
	t.Cleanup(func() { os.Unsetenv("GUST_EXEC_HELPER") })

	run, err := r.Run(context.Background(), api.TestScenario{
		ID:   "exec",
		Task: api.TaskInfo{ID: "t", Input: "do it"},
	}, "http://fixtures")
	if err != nil {
		t.Fatal(err)
	}
	if run.Metadata["otel_protocol"] != "http/protobuf" {
		t.Fatalf("protocol %v", run.Metadata["otel_protocol"])
	}
	if run.Metadata["otel_traces"] != "http://127.0.0.1:4318/v1/traces" {
		t.Fatalf("traces %v", run.Metadata["otel_traces"])
	}
	res, _ := run.Metadata["resource"].(string)
	if !strings.Contains(res, "gust.sample_id=") {
		t.Fatalf("resource %q", res)
	}
}
