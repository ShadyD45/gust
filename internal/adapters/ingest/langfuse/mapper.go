// Package langfuse maps Langfuse public-API traces onto gust AgentRun documents.
package langfuse

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gust/pkg/api"
)

// Trace is the subset of a Langfuse public trace we need.
type Trace struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	SessionID    string         `json:"sessionId"`
	Timestamp    string         `json:"timestamp"`
	Input        any            `json:"input"`
	Output       any            `json:"output"`
	Metadata     map[string]any `json:"metadata"`
	Observations []Observation  `json:"observations"`
}

// Observation is one Langfuse span / generation / event.
type Observation struct {
	ID                  string         `json:"id"`
	TraceID             string         `json:"traceId"`
	ParentObservationID string         `json:"parentObservationId"`
	Type                string         `json:"type"`
	Name                string         `json:"name"`
	StartTime           string         `json:"startTime"`
	EndTime             string         `json:"endTime"`
	Input               any            `json:"input"`
	Output              any            `json:"output"`
	Model               string         `json:"model"`
	Level               string         `json:"level"`
	StatusMessage       string         `json:"statusMessage"`
	Metadata            map[string]any `json:"metadata"`
}

// TraceList is GET /api/public/traces.
type TraceList struct {
	Data []Trace `json:"data"`
}

// Options override derived AgentRun fields.
type Options struct {
	RunID        string
	AgentName    string
	AgentVersion string
	TaskID       string
	TaskInput    string
}

// Map converts a Langfuse trace into an AgentRun.
func Map(tr Trace, opts Options) (api.AgentRun, error) {
	if tr.ID == "" && len(tr.Observations) == 0 {
		return api.AgentRun{}, fmt.Errorf("langfuse: trace has no id or observations")
	}
	runID := firstNonEmpty(opts.RunID, tr.ID)
	if runID == "" {
		return api.AgentRun{}, fmt.Errorf("langfuse: trace id is required")
	}

	meta := tr.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	agentName := firstNonEmpty(opts.AgentName, stringMeta(meta, "service.name"), "unknown-agent")
	agentVer := firstNonEmpty(opts.AgentVersion, stringMeta(meta, "service.version"), "unknown")
	taskID := firstNonEmpty(opts.TaskID, tr.SessionID, stringMeta(meta, "session.id"), runID)
	taskInput := firstNonEmpty(opts.TaskInput, stringify(tr.Input), tr.Name)
	if taskInput == "" {
		taskInput = runID
	}

	spans := make([]api.Span, 0, len(tr.Observations))
	for _, obs := range tr.Observations {
		if obs.ID == "" || obs.Name == "" {
			return api.AgentRun{}, fmt.Errorf("langfuse: observation missing id or name")
		}
		span := api.Span{
			SpanID:       obs.ID,
			ParentSpanID: obs.ParentObservationID,
			Name:         spanName(obs),
			Type:         spanType(obs),
			StartTime:    parseTime(obs.StartTime),
			EndTime:      parseTime(firstNonEmpty(obs.EndTime, obs.StartTime)),
			Attributes:   spanAttrs(obs),
			Status:       spanStatus(obs),
		}
		spans = append(spans, span)
	}

	status := "completed"
	if strings.EqualFold(stringMeta(meta, "status"), "error") {
		status = "failed"
	}

	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         runID,
		Agent: api.AgentInfo{
			Name:      agentName,
			Version:   agentVer,
			GitCommit: stringMeta(meta, "service.git.commit"),
		},
		Task: api.TaskInfo{
			ID:    taskID,
			Input: taskInput,
		},
		Trace: spans,
		Outcome: api.RunOutcome{
			Status: status,
			Output: stringify(tr.Output),
		},
		Metadata: map[string]any{"source": "langfuse"},
	}
	if err := run.Validate(); err != nil {
		return api.AgentRun{}, fmt.Errorf("langfuse: mapped run invalid: %w", err)
	}
	return run, nil
}

func spanName(obs Observation) string {
	if tool := stringMeta(obs.Metadata, "tool.name"); tool != "" {
		return tool
	}
	return obs.Name
}

func spanType(obs Observation) api.SpanType {
	kind := strings.ToUpper(stringMeta(obs.Metadata, "openinference.span.kind"))
	switch kind {
	case "TOOL":
		return api.SpanTypeTool
	case "LLM":
		return api.SpanTypeLLM
	case "RETRIEVER":
		return api.SpanTypeRetrieval
	}
	if stringMeta(obs.Metadata, "tool.name") != "" || obs.Metadata["tool.parameters"] != nil {
		return api.SpanTypeTool
	}
	switch strings.ToUpper(obs.Type) {
	case "GENERATION":
		return api.SpanTypeLLM
	default:
		return api.SpanTypeAgent
	}
}

func spanAttrs(obs Observation) map[string]any {
	attrs := map[string]any{}
	if in := obs.Input; in != nil {
		attrs["input"] = coerceInput(in, obs.Metadata)
	} else if params := obs.Metadata["tool.parameters"]; params != nil {
		attrs["input"] = coerceJSON(params)
	}
	if obs.Output != nil {
		attrs["output"] = obs.Output
	}
	if obs.Model != "" {
		attrs["model"] = obs.Model
	}
	return attrs
}

func coerceInput(in any, meta map[string]any) any {
	if params := meta["tool.parameters"]; params != nil {
		return coerceJSON(params)
	}
	return coerceJSON(in)
}

func coerceJSON(v any) any {
	switch t := v.(type) {
	case string:
		var parsed any
		if json.Unmarshal([]byte(t), &parsed) == nil {
			return parsed
		}
		return t
	default:
		return v
	}
}

func spanStatus(obs Observation) api.SpanStatus {
	if strings.EqualFold(obs.Level, "ERROR") {
		return api.SpanStatus{Code: "error", Message: obs.StatusMessage}
	}
	return api.SpanStatus{Code: "ok"}
}

func stringify(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprint(t)
		}
		return string(b)
	}
}

func stringMeta(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	v, ok := meta[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if ts, err := time.Parse(layout, s); err == nil {
			return ts
		}
	}
	return time.Time{}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
