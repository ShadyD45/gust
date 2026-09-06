package scenario

import (
	"fmt"
	"strings"
	"time"

	"gust/pkg/api"
	"gust/pkg/jcs"
)

// Extractor converts AgentRun traces into TestScenario drafts without assertions (H8).
type Extractor struct{}

// NewExtractor creates a scenario extractor.
func NewExtractor() *Extractor { return &Extractor{} }

// ExtractFromRun builds a TestScenario with empty assertions and recorded fixtures.
func (e *Extractor) ExtractFromRun(run api.AgentRun) (*api.TestScenario, error) {
	sc := &api.TestScenario{
		ID:          "scenario_" + sanitizeID(run.RunID),
		Version:     "1.0",
		Description: "TODO: Add description of expected behavior for this scenario.",
		Task:        run.Task,
		Environment: api.EnvironmentSpec{
			FixtureStrategy: api.MatchStrategyHybrid,
			Fixtures:        make([]api.Fixture, 0),
		},
		// CRITICAL: Assertions intentionally empty — never synthesize from trace (H8).
		Assertions: []api.Assertion{},
		Reliability: api.ReliabilityConfig{
			Samples:         10,
			MinimumPassRate: 0.95,
			Confidence:      0.95,
		},
		Provenance: api.TestScenarioProvenance{
			Source:      "production_trace",
			SourceRunID: run.RunID,
			ExtractedAt: time.Now().UTC(),
			ReviewedBy:  "",
		},
	}

	for i, span := range run.Trace {
		if span.Type != api.SpanTypeTool {
			continue
		}
		inputMap, _ := span.Attributes["input"].(map[string]any)
		if inputMap == nil {
			inputMap = map[string]any{}
		}
		hash, err := jcs.ContentHash(inputMap)
		if err != nil {
			return nil, fmt.Errorf("hash fixture input for span %s: %w", span.SpanID, err)
		}
		outputBody := span.Attributes["output"]
		fx := api.Fixture{
			FixtureID:     fmt.Sprintf("fx_%s_%03d", sanitizeID(span.Name), i+1),
			Tool:          span.Name,
			InputHash:     hash,
			MatchStrategy: api.MatchStrategyExactHash,
			RecordedInput: inputMap,
			RecordedResponse: api.RecordedResponse{
				Status: "success",
				Body:   outputBody,
			},
			Provenance: api.ProvenanceRecorded,
		}
		sc.Environment.Fixtures = append(sc.Environment.Fixtures, fx)
	}

	return sc, nil
}

func sanitizeID(s string) string {
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "/", "_")
	return s
}
