package api

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAgentRunValidation(t *testing.T) {
	now := time.Now().UTC()
	validRun := AgentRun{
		SchemaVersion: SchemaVersion,
		RunID:         "run_123",
		Agent: AgentInfo{
			Name:    "support-agent",
			Version: "1.0.0",
		},
		Task: TaskInfo{
			ID:    "task_001",
			Input: "Refund order 123",
		},
		Trace: []Span{
			{
				SpanID:    "span_1",
				Name:      "root",
				Type:      SpanTypeAgent,
				StartTime: now,
				EndTime:   now.Add(100 * time.Millisecond),
				Status:    SpanStatus{Code: "ok"},
			},
		},
		Outcome: RunOutcome{
			Status: "completed",
			Output: "Order 123 has been refunded.",
		},
	}

	if err := validRun.Validate(); err != nil {
		t.Fatalf("expected valid run, got error: %v", err)
	}

	// Test invalid schema version
	invalidRun := validRun
	invalidRun.SchemaVersion = "0.1"
	if err := invalidRun.Validate(); err == nil {
		t.Errorf("expected error for schema version 0.1, got nil")
	}

	// Test missing agent name
	invalidRun = validRun
	invalidRun.Agent.Name = ""
	if err := invalidRun.Validate(); err == nil {
		t.Errorf("expected error for empty agent name, got nil")
	}

	// Test invalid span type
	invalidSpan := validRun.Trace[0]
	invalidSpan.Type = "unknown_type"
	if err := ValidateSpan(&invalidSpan, 0); err == nil {
		t.Errorf("expected error for invalid span type, got nil")
	}
}

func TestTestScenarioValidation(t *testing.T) {
	scenario := TestScenario{
		ID:          "cancel_order_test",
		Version:     "1.0",
		Description: "Tests order cancellation",
		Task: TaskInfo{
			Input: "Cancel order 123",
		},
		Reliability: ReliabilityConfig{
			Samples:         20,
			MinimumPassRate: 0.95,
			Confidence:      0.95,
		},
		Provenance: TestScenarioProvenance{
			Source:      "production_trace",
			ExtractedAt: time.Now().UTC(),
		},
	}

	if err := scenario.Validate(); err != nil {
		t.Fatalf("expected valid scenario, got error: %v", err)
	}

	// Test invalid minimum pass rate
	badRate := scenario
	badRate.Reliability.MinimumPassRate = 1.5
	if err := badRate.Validate(); err == nil {
		t.Errorf("expected error for pass rate > 1.0, got nil")
	}
}

func TestFixtureValidation(t *testing.T) {
	fx := Fixture{
		FixtureID: "fx_1",
		Tool:      "get_orders",
		InputHash: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		RecordedResponse: RecordedResponse{
			Status: "success",
			Body:   map[string]any{"id": 123},
		},
		Provenance: ProvenanceRecorded,
	}

	if err := ValidateFixtureObject(&fx); err != nil {
		t.Fatalf("expected valid fixture, got error: %v", err)
	}

	// Test invalid hash format
	badHash := fx
	badHash.InputHash = "md5:12345"
	if err := ValidateFixtureObject(&badHash); err == nil {
		t.Errorf("expected error for non-sha256 hash pattern, got nil")
	}
}

func TestPolicyValidation(t *testing.T) {
	policy := Policy{
		Version: "1.0",
		Name:    "ci-gate",
		Reliability: PolicyReliability{
			DefaultMinimumPassRate: 0.95,
			MinSamplesForVerdict:   5,
			OnFlaky:                "warn",
		},
	}

	if err := policy.Validate(); err != nil {
		t.Fatalf("expected valid policy, got: %v", err)
	}

	// Test bad on_flaky value
	badPolicy := policy
	badPolicy.Reliability.OnFlaky = "explode"
	if err := badPolicy.Validate(); err == nil {
		t.Errorf("expected error for on_flaky 'explode', got nil")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	run := AgentRun{
		SchemaVersion: SchemaVersion,
		RunID:         "run_abc",
		Agent:         AgentInfo{Name: "agent", Version: "1.0"},
		Task:          TaskInfo{ID: "t1", Input: "hello"},
		Outcome:       RunOutcome{Status: "completed", Output: "world"},
	}

	data, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded AgentRun
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.RunID != run.RunID || decoded.Outcome.Output != "world" {
		t.Errorf("decoded struct mismatch: %+v", decoded)
	}
}

func TestValidateJSONHelper(t *testing.T) {
	rawJSON := `{
		"schema_version": "0.5",
		"run_id": "run_test_json",
		"agent": { "name": "bot", "version": "1.0" },
		"task": { "id": "task_1", "input": "test task" },
		"outcome": { "status": "completed" }
	}`

	var run AgentRun
	if err := ValidateJSON([]byte(rawJSON), &run); err != nil {
		t.Fatalf("ValidateJSON failed on valid json: %v", err)
	}

	badJSON := `{ "schema_version": "0.1" }`
	if err := ValidateJSON([]byte(badJSON), &run); err == nil {
		t.Errorf("ValidateJSON should fail on bad data, got nil")
	} else if !strings.Contains(err.Error(), "invalid or unsupported schema version") && !strings.Contains(err.Error(), "run_id is required") {
		t.Logf("expected validation error, got: %v", err)
	}
}
