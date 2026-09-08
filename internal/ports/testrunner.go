package ports

import (
	"context"

	"gust/pkg/api"
)

// SampleRequest is the Gust-created context for one Mode 3 sample.
// Trigger, world control, and observation adapters all read from this contract.
type SampleRequest struct {
	EvaluationID    string
	ScenarioID      string
	SampleID        string
	TraceID         string
	TraceParent     string
	Baggage         string
	Scenario        api.TestScenario
	FixtureEndpoint string
	OTelEndpoint    string
	IngestURL       string
	WorldMode       api.WorldControlMode
}

// FixtureEndpointOrEmpty returns the fixture proxy URL when Gust world control is active.
func (r SampleRequest) FixtureEndpointOrEmpty() string {
	if r.WorldMode == api.WorldControlExisting {
		return ""
	}
	return r.FixtureEndpoint
}

// TestRunner executes one live sample against the agent under test.
type TestRunner interface {
	Name() string
	Run(ctx context.Context, req SampleRequest) (api.AgentRun, error)
}

// ExecutionReceipt is an optional completion signal from a trigger harness.
// When TraceID is set, a TraceSource can fetch the run from an existing backend.
type ExecutionReceipt struct {
	Status  string         `json:"status"` // completed | failed | timeout | cancelled
	TraceID string         `json:"trace_id,omitempty"`
	RunID   string         `json:"run_id,omitempty"`
	Error   string         `json:"error,omitempty"`
	Run     *api.AgentRun  `json:"run,omitempty"`
	Meta    map[string]any `json:"metadata,omitempty"`
}

// TraceSource retrieves or waits for an AgentRun after a sample is triggered.
type TraceSource interface {
	Name() string
	Fetch(ctx context.Context, sampleID, traceID string) (api.AgentRun, error)
}
