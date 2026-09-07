package validation

import (
	"fmt"
	"time"

	"gust/pkg/api"
	"gust/pkg/jcs"
)

func now() time.Time {
	return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
}

func baseRun(id string) api.AgentRun {
	return api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         id,
		Agent:         api.AgentInfo{Name: "validation-agent", Version: "1.0"},
		Task:          api.TaskInfo{ID: "task_" + id, Input: "do the thing"},
		Trace:         nil,
		Outcome:       api.RunOutcome{Status: "completed", Output: "ok"},
		Metadata:      map[string]any{"suite": "gust_validation"},
	}
}

func withTrace(run api.AgentRun, spans ...api.Span) api.AgentRun {
	run.Trace = spans
	return run
}

func toolSpan(id, name string, input map[string]any, ok bool) api.Span {
	t := now()
	st := api.SpanStatus{Code: "ok"}
	if !ok {
		st = api.SpanStatus{Code: "error", Message: "tool failed"}
	}
	attrs := map[string]any{}
	if input != nil {
		attrs["input"] = input
	}
	return api.Span{
		SpanID:     id,
		Name:       name,
		Type:       api.SpanTypeTool,
		StartTime:  t,
		EndTime:    t.Add(5 * time.Millisecond),
		Attributes: attrs,
		Status:     st,
	}
}

func llmSpan(id string) api.Span {
	t := now()
	return api.Span{
		SpanID:    id,
		Name:      "llm",
		Type:      api.SpanTypeLLM,
		StartTime: t,
		EndTime:   t.Add(2 * time.Millisecond),
		Status:    api.SpanStatus{Code: "ok"},
	}
}

func agentSpan(id string) api.Span {
	t := now()
	return api.Span{
		SpanID:    id,
		Name:      "agent",
		Type:      api.SpanTypeAgent,
		StartTime: t,
		EndTime:   t.Add(1 * time.Millisecond),
		Status:    api.SpanStatus{Code: "ok"},
	}
}

func errorSpan(id, name, msg string) api.Span {
	t := now()
	return api.Span{
		SpanID:    id,
		Name:      name,
		Type:      api.SpanTypeError,
		StartTime: t,
		EndTime:   t.Add(3 * time.Millisecond),
		Status:    api.SpanStatus{Code: "error", Message: msg},
	}
}

func mustHash(v map[string]any) string {
	h, err := jcs.ContentHash(v)
	if err != nil {
		panic(fmt.Sprintf("hash: %v", err))
	}
	return h
}

func a(id string, typ api.AssertionType, opts ...func(*api.Assertion)) api.Assertion {
	x := api.Assertion{ID: id, Type: typ}
	for _, o := range opts {
		o(&x)
	}
	return x
}

func withTool(tool string) func(*api.Assertion) {
	return func(x *api.Assertion) { x.Tool = tool }
}

func withArgs(args map[string]any) func(*api.Assertion) {
	return func(x *api.Assertion) { x.Arguments = args }
}

func withLimit(n int) func(*api.Assertion) {
	return func(x *api.Assertion) { x.Limit = n }
}

func withParams(p map[string]any) func(*api.Assertion) {
	return func(x *api.Assertion) { x.Parameters = p }
}
