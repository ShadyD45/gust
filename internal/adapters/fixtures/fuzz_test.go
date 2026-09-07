package fixtures

import (
	"context"
	"encoding/json"
	"testing"

	"gust/internal/ports"
	"gust/pkg/api"
)

func FuzzFixtureLookup(f *testing.F) {
	f.Add("echo", `{"msg":"hi"}`)
	f.Add("poll", `{}`)
	f.Fuzz(func(t *testing.T, tool, argsJSON string) {
		var args map[string]any
		if json.Unmarshal([]byte(argsJSON), &args) != nil {
			args = map[string]any{"raw": argsJSON}
		}
		p := NewMemoryFixtureProvider()
		_ = p.LoadFixtures([]api.Fixture{
			{
				FixtureID:        "fx",
				Tool:             "echo",
				RecordedInput:    map[string]any{"msg": "hi"},
				RecordedResponse: api.RecordedResponse{Status: "success", Body: "ok"},
				Provenance:       api.ProvenanceRecorded,
			},
			{
				FixtureID:        "fx_seq",
				Tool:             "poll",
				MatchStrategy:    api.MatchStrategyOrderedSequence,
				RecordedResponse: api.RecordedResponse{Status: "success", Body: "PENDING"},
				Provenance:       api.ProvenanceRecorded,
			},
		})
		_, _, _ = p.Lookup(context.Background(), ports.ToolCall{Name: tool, Arguments: args})
	})
}
