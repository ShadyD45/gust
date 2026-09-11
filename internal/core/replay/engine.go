package replay

import (
	"context"
	"fmt"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

// ReplayEngine reconstructs a recorded agent trajectory against deterministic fixtures.
// It does not invoke the LLM or make the agent choose tools again — use Test mode for that.
type ReplayEngine struct {
	fixtures ports.FixtureProvider
}

// NewReplayEngine initializes a ReplayEngine with the provided fixture provider.
func NewReplayEngine(fixtures ports.FixtureProvider) *ReplayEngine {
	return &ReplayEngine{fixtures: fixtures}
}

// ReplayTrace reproduces an AgentRun by re-resolving each tool call against fixtures.
func (e *ReplayEngine) ReplayTrace(ctx context.Context, original api.AgentRun) (api.AgentRun, error) {
	_ = e.fixtures.Reset()

	replayed := api.AgentRun{
		SchemaVersion: original.SchemaVersion,
		RunID:         fmt.Sprintf("replay_%s", original.RunID),
		Agent:         original.Agent,
		Task:          original.Task,
		Metadata:      original.Metadata,
		Trace:         make([]api.Span, 0, len(original.Trace)),
		Outcome:       original.Outcome,
	}

	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for i, sp := range original.Trace {
		newSpan := sp
		// Deterministic timestamps for replay reproducibility
		newSpan.StartTime = baseTime.Add(time.Duration(i*10) * time.Millisecond)
		newSpan.EndTime = baseTime.Add(time.Duration(i*10+5) * time.Millisecond)

		if sp.Type == api.SpanTypeTool {
			inputMap, _ := sp.Attributes["input"].(map[string]any)
			call := ports.ToolCall{
				Name:      sp.Name,
				Arguments: inputMap,
			}

			resp, found, err := e.fixtures.Lookup(ctx, call)
			if err != nil {
				return api.AgentRun{}, fmt.Errorf("replay fixture error on %q: %w", sp.Name, err)
			}
			if !found {
				return api.AgentRun{}, fmt.Errorf("replay fixture missing for tool %q", sp.Name)
			}

			// Clone attributes and assign replayed output
			attrs := make(map[string]any)
			for k, v := range sp.Attributes {
				attrs[k] = v
			}
			attrs["output"] = resp.Body
			newSpan.Attributes = attrs

			if resp.Status == "error" {
				newSpan.Status = api.SpanStatus{Code: "error", Message: resp.Error}
			} else {
				newSpan.Status = api.SpanStatus{Code: "ok"}
			}
		}

		replayed.Trace = append(replayed.Trace, newSpan)
	}

	return replayed, nil
}
