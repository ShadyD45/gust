package testrunner

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

const (
	EnvEvaluationID = "GUST_EVALUATION_ID"
	EnvScenarioID   = "GUST_SCENARIO_ID"
	EnvTraceID      = "GUST_TRACE_ID"
	EnvTraceParent  = "TRACEPARENT"
	EnvBaggage      = "BAGGAGE"
	EnvWorldMode    = "GUST_WORLD_CONTROL"
	EnvTaskInput    = "AGENTEVAL_TASK_INPUT"
)

// NewEvaluationID creates a suite-scoped evaluation identifier.
func NewEvaluationID() string {
	return fmt.Sprintf("eval-%d-%s", time.Now().UnixNano(), randomHex(4))
}

// NewSampleID creates a scenario-scoped sample identifier.
func NewSampleID(scenarioID string) string {
	return fmt.Sprintf("%s-%d-%s", scenarioID, time.Now().UnixNano(), randomHex(4))
}

// NewTraceID returns a 16-byte W3C trace id (32 hex chars).
func NewTraceID() string {
	return randomHex(16)
}

// BuildTraceParent builds a W3C traceparent for a known trace id.
func BuildTraceParent(traceID string) string {
	if traceID == "" {
		traceID = NewTraceID()
	}
	spanID := randomHex(8)
	return fmt.Sprintf("00-%s-%s-01", traceID, spanID)
}

// BuildBaggage encodes Gust correlation keys for W3C baggage propagation.
func BuildBaggage(evaluationID, scenarioID, sampleID string) string {
	parts := make([]string, 0, 3)
	if evaluationID != "" {
		parts = append(parts, "gust.evaluation_id="+evaluationID)
	}
	if scenarioID != "" {
		parts = append(parts, "gust.scenario_id="+scenarioID)
	}
	if sampleID != "" {
		parts = append(parts, "gust.sample_id="+sampleID)
	}
	return strings.Join(parts, ",")
}

// NewSampleRequest builds a SampleRequest for one Mode 3 invocation.
func NewSampleRequest(evaluationID string, scenario api.TestScenario, fixtureEndpoint, otelURL string) ports.SampleRequest {
	sampleID := NewSampleID(scenario.ID)
	traceID := NewTraceID()
	ingestURL := ""
	if otelURL != "" {
		ingestURL = strings.TrimRight(otelURL, "/") + "/v1/runs"
	}
	world := scenario.Environment.EffectiveWorldControl()
	if world == api.WorldControlExisting {
		fixtureEndpoint = ""
	}
	return ports.SampleRequest{
		EvaluationID:    evaluationID,
		ScenarioID:      scenario.ID,
		SampleID:        sampleID,
		TraceID:         traceID,
		TraceParent:     BuildTraceParent(traceID),
		Baggage:         BuildBaggage(evaluationID, scenario.ID, sampleID),
		Scenario:        scenario,
		FixtureEndpoint: fixtureEndpoint,
		OTelEndpoint:    otelURL,
		IngestURL:       ingestURL,
		WorldMode:       world,
	}
}

// normalizeSampleRequest fills identity fields while preserving explicit caller overrides.
func normalizeSampleRequest(req ports.SampleRequest, otelFallback string) ports.SampleRequest {
	base := NewSampleRequest(req.EvaluationID, req.Scenario, req.FixtureEndpoint, firstNonEmpty(req.OTelEndpoint, otelFallback))
	if req.SampleID != "" {
		base.SampleID = req.SampleID
	}
	if req.TraceID != "" {
		base.TraceID = req.TraceID
		base.TraceParent = BuildTraceParent(req.TraceID)
	}
	if req.TraceParent != "" {
		base.TraceParent = req.TraceParent
	}
	if req.Baggage != "" {
		base.Baggage = req.Baggage
	}
	if req.WorldMode != "" {
		base.WorldMode = req.WorldMode
		if req.WorldMode == api.WorldControlGust {
			base.FixtureEndpoint = req.FixtureEndpoint
		} else {
			base.FixtureEndpoint = ""
		}
	}
	if req.EvaluationID != "" {
		base.EvaluationID = req.EvaluationID
	}
	if req.ScenarioID != "" {
		base.ScenarioID = req.ScenarioID
	}
	if req.IngestURL != "" {
		base.IngestURL = req.IngestURL
	}
	base.Baggage = BuildBaggage(base.EvaluationID, base.ScenarioID, base.SampleID)
	return base
}

func stampSampleMetadata(run *api.AgentRun, req ports.SampleRequest) {
	if run.Metadata == nil {
		run.Metadata = map[string]any{}
	}
	if req.SampleID != "" {
		run.Metadata["sample_id"] = req.SampleID
	}
	if req.EvaluationID != "" {
		run.Metadata["evaluation_id"] = req.EvaluationID
	}
	if req.ScenarioID != "" {
		run.Metadata["scenario_id"] = req.ScenarioID
	}
	if req.TraceID != "" {
		run.Metadata["trace_id"] = req.TraceID
	}
	if req.WorldMode != "" {
		run.Metadata["world_control"] = string(req.WorldMode)
	}
}

func applySampleEnv(environ []string, req ports.SampleRequest) []string {
	fixture := req.FixtureEndpointOrEmpty()
	environ = upsertEnv(environ, EnvFixtureEndpoint, fixture)
	environ = upsertEnv(environ, EnvSampleID, req.SampleID)
	environ = upsertEnv(environ, EnvEvaluationID, req.EvaluationID)
	environ = upsertEnv(environ, EnvScenarioID, req.ScenarioID)
	environ = upsertEnv(environ, EnvTraceID, req.TraceID)
	environ = upsertEnv(environ, EnvTraceParent, req.TraceParent)
	environ = upsertEnv(environ, EnvBaggage, req.Baggage)
	environ = upsertEnv(environ, EnvWorldMode, string(req.WorldMode))
	environ = upsertEnv(environ, EnvTaskInput, req.Scenario.Task.Input)
	environ = ApplyOTelEnv(environ, req.OTelEndpoint, req.SampleID)
	if req.EvaluationID != "" {
		environ = mergeResourceAttribute(environ, "gust.evaluation_id="+req.EvaluationID)
	}
	if req.ScenarioID != "" {
		environ = mergeResourceAttribute(environ, "gust.scenario_id="+req.ScenarioID)
	}
	if req.TraceID != "" {
		environ = mergeResourceAttribute(environ, "gust.trace_id="+req.TraceID)
	}
	return environ
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
