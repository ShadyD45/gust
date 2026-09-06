package replay

import (
	"context"
	"testing"
	"time"

	"gust/internal/adapters/fixtures"
	"gust/pkg/api"
	"gust/pkg/jcs"
)

func TestReplayDeterminism(t *testing.T) {
	provider := fixtures.NewMemoryFixtureProvider()
	_ = provider.LoadFixtures([]api.Fixture{
		{
			FixtureID: "fx_1",
			Tool:      "calc",
			RecordedInput: map[string]any{
				"expr": "2+2",
			},
			RecordedResponse: api.RecordedResponse{
				Status: "success",
				Body:   4,
			},
			Provenance: api.ProvenanceRecorded,
		},
	})

	originalRun := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "golden_run",
		Agent:         api.AgentInfo{Name: "agent", Version: "1.0"},
		Task:          api.TaskInfo{ID: "t1", Input: "Calculate 2+2"},
		Trace: []api.Span{
			{
				SpanID:    "span_1",
				Name:      "calc",
				Type:      api.SpanTypeTool,
				StartTime: time.Now(),
				EndTime:   time.Now().Add(10 * time.Millisecond),
				Attributes: map[string]any{
					"input": map[string]any{"expr": "2+2"},
				},
				Status: api.SpanStatus{Code: "ok"},
			},
		},
		Outcome: api.RunOutcome{Status: "completed", Output: "4"},
	}

	engine := NewReplayEngine(provider)
	ctx := context.Background()

	firstReplay, err := engine.ReplayTrace(ctx, originalRun)
	if err != nil {
		t.Fatalf("first replay failed: %v", err)
	}

	expectedHash, err := jcs.ContentHash(firstReplay)
	if err != nil {
		t.Fatalf("content hash failed: %v", err)
	}

	// Replay 100 times and verify identical hash
	for i := 0; i < 100; i++ {
		replayed, err := engine.ReplayTrace(ctx, originalRun)
		if err != nil {
			t.Fatalf("replay %d failed: %v", i, err)
		}
		hash, err := jcs.ContentHash(replayed)
		if err != nil {
			t.Fatalf("hash %d failed: %v", i, err)
		}
		if hash != expectedHash {
			t.Fatalf("determinism violation on run %d: got %s, want %s", i, hash, expectedHash)
		}
	}
}
