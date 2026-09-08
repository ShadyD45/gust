package testrunner

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

func TestTriggerRunner_ReceiptWithInlineRun(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	os.Setenv("GUST_TRIGGER_HELPER", "1")
	t.Cleanup(func() { os.Unsetenv("GUST_TRIGGER_HELPER") })

	r := NewTriggerRunner([]string{exe}, CollectorConfig{
		Source:      TraceSourceResponse,
		WaitTimeout: 5 * time.Second,
	})
	r.Timeout = 10 * time.Second
	run, err := r.Run(context.Background(), ports.SampleRequest{
		Scenario:  api.TestScenario{ID: "trig", Task: api.TaskInfo{ID: "t", Input: "hi"}},
		WorldMode: api.WorldControlExisting,
	})
	if err != nil {
		t.Fatal(err)
	}
	if run.Outcome.Status != "completed" {
		t.Fatalf("status %s", run.Outcome.Status)
	}
	if run.Metadata["sample_id"] == nil {
		t.Fatal("expected sample metadata")
	}
}

func writeTriggerHelperReceipt() {
	run := validRun(os.Getenv(EnvSampleID))
	receipt := ports.ExecutionReceipt{
		Status:  "completed",
		TraceID: "tid-1",
		Run:     &run,
	}
	_ = json.NewEncoder(os.Stdout).Encode(receipt)
}
