package ports

import (
	"context"
	"gust/pkg/api"
)

// ToolCall represents a runtime tool invocation from an agent.
type ToolCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
	CallID    string         `json:"call_id,omitempty"`
}

// FixtureProvider resolves recorded tool responses during Replay and Test modes.
type FixtureProvider interface {
	Lookup(ctx context.Context, call ToolCall) (api.RecordedResponse, bool, error)
	Record(ctx context.Context, call ToolCall, resp api.RecordedResponse) error
	Reset() error
}
