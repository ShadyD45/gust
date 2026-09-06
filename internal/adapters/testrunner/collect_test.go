package testrunner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gust/pkg/api"
)

func validRun(id string) api.AgentRun {
	now := time.Now().UTC()
	return api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         id,
		Agent:         api.AgentInfo{Name: "agent", Version: "1"},
		Task:          api.TaskInfo{ID: "t", Input: "do it"},
		Trace: []api.Span{{
			SpanID:    "s1",
			Name:      "step",
			Type:      api.SpanTypeAgent,
			StartTime: now,
			EndTime:   now.Add(time.Millisecond),
			Status:    api.SpanStatus{Code: "ok"},
		}},
		Outcome: api.RunOutcome{Status: "completed", Output: "ok"},
	}
}

func TestCollect_Response(t *testing.T) {
	run := validRun("r1")
	body, _ := json.Marshal(run)
	got, err := Collect(context.Background(), CollectorConfig{Source: TraceSourceResponse}, "s1", body)
	if err != nil {
		t.Fatal(err)
	}
	if got.RunID != "r1" {
		t.Fatalf("run id %s", got.RunID)
	}
}

func TestCollect_InvalidResponse(t *testing.T) {
	_, err := Collect(context.Background(), CollectorConfig{Source: TraceSourceResponse}, "s1", []byte(`{"nope":true}`))
	if err == nil {
		t.Fatal("expected validate error")
	}
}

func TestCollect_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "{sample_id}.json")
	run := validRun("from-file")
	data, _ := json.Marshal(run)
	if err := os.WriteFile(filepath.Join(dir, "abc.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	got, err := Collect(context.Background(), CollectorConfig{
		Source:       TraceSourceFile,
		PathTemplate: path,
		WaitTimeout:  time.Second,
	}, "abc", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.RunID != "from-file" {
		t.Fatalf("run id %s", got.RunID)
	}
}

func TestCollect_AutoPrefersResponse(t *testing.T) {
	run := validRun("resp")
	body, _ := json.Marshal(run)
	got, err := Collect(context.Background(), CollectorConfig{Source: TraceSourceAuto}, "s", body)
	if err != nil {
		t.Fatal(err)
	}
	if got.RunID != "resp" {
		t.Fatalf("run id %s", got.RunID)
	}
}
