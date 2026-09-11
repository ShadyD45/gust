package redact

import (
	"strings"
	"testing"

	"gust/pkg/api"
)

func TestRedactEmailAndToken(t *testing.T) {
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "r1",
		Agent:         api.AgentInfo{Name: "a", Version: "1"},
		Task:          api.TaskInfo{ID: "t", Input: "contact alice@example.com with Bearer tok_abcdefghijklmnopqrst and sk-abcdefghijklmnopqrstuvwxyz"},
		Trace: []api.Span{{
			SpanID: "1", Name: "send_email", Type: api.SpanTypeTool,
			Attributes: map[string]any{"input": map[string]any{"to": "bob@corp.test"}},
		}},
		Outcome: api.RunOutcome{Status: "completed"},
	}
	out, res, err := RedactRun(run, Config{Enabled: true, Rules: DefaultRules()})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalReplaced == 0 {
		t.Fatalf("expected replacements, got %+v", res)
	}
	if strings.Contains(out.Task.Input, "alice@example.com") {
		t.Fatalf("email still present: %q", out.Task.Input)
	}
	if strings.Contains(out.Task.Input, "sk-abcdefghijklmnopqrstuvwxyz") {
		t.Fatalf("api key still present: %q", out.Task.Input)
	}
}

func TestRedactOptOut(t *testing.T) {
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "r1",
		Agent:         api.AgentInfo{Name: "a", Version: "1"},
		Task:          api.TaskInfo{ID: "t", Input: "alice@example.com"},
		Outcome:       api.RunOutcome{Status: "completed"},
	}
	out, res, err := RedactRun(run, Config{Enabled: false, OptOutNote: "staging synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Enabled || res.OptOutNote == "" {
		t.Fatalf("expected audited opt-out, got %+v", res)
	}
	if out.Task.Input != "alice@example.com" {
		t.Fatalf("opt-out must leave text unchanged")
	}
}
