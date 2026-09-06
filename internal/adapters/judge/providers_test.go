package judge_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gust/internal/adapters/judge"
	"gust/internal/ports"
	"gust/pkg/api"
)

func TestGenericProviderChatCompletions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": `{"score": 0.9, "passed": true, "rationale": "good"}`}},
			},
		})
	}))
	defer srv.Close()

	p := judge.NewGenericProvider(srv.URL, "sk-test", "test-model")
	res, err := p.Judge(context.Background(), ports.JudgeRequest{
		Run: api.AgentRun{
			SchemaVersion: api.SchemaVersion,
			RunID:         "r1",
			Agent:         api.AgentInfo{Name: "a", Version: "1"},
			Task:          api.TaskInfo{ID: "t", Input: "hi"},
			Outcome:       api.RunOutcome{Status: "completed", Output: "hello"},
		},
		Rubric:    "be helpful",
		Threshold: 0.7,
	})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if !res.Passed || res.Score < 0.89 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestResolveProvider(t *testing.T) {
	p, err := judge.ResolveProvider("mock", "", "", "")
	if err != nil || p.Name() != "mock" {
		t.Fatalf("ResolveProvider mock: %v %#v", err, p)
	}
	p, err = judge.ResolveProvider("generic", "http://localhost:9", "", "m")
	if err != nil || p.Name() != "generic" {
		t.Fatalf("ResolveProvider generic: %v %#v", err, p)
	}
	_, err = judge.ResolveProvider("openai", "", "", "")
	if err == nil {
		t.Fatal("expected openai to require Python SDK plugin")
	}
	_, err = judge.ResolveProvider("nope", "", "", "")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestMockProvider(t *testing.T) {
	p := &judge.MockProvider{}
	res, err := p.Judge(context.Background(), ports.JudgeRequest{
		Run:       api.AgentRun{Outcome: api.RunOutcome{Status: "completed", Output: "Order cancelled successfully with confirmation id."}},
		Rubric:    "ok",
		Threshold: 0.7,
	})
	if err != nil || !res.Passed {
		t.Fatalf("mock: %+v %v", res, err)
	}
}
