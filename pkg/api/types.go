package api

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// SchemaVersion defines the current specification version.
const SchemaVersion = "0.5"

// Common errors
var (
	ErrInvalidSchemaVersion = errors.New("invalid or unsupported schema version")
	ErrMissingRequiredField = errors.New("missing required field")
	ErrInvalidFieldValue    = errors.New("invalid field value")
)

// AgentRun represents a captured or recorded execution trace.
type AgentRun struct {
	SchemaVersion string         `json:"schema_version"`
	RunID         string         `json:"run_id"`
	Agent         AgentInfo      `json:"agent"`
	Task          TaskInfo       `json:"task"`
	Trace         []Span         `json:"trace"`
	Outcome       RunOutcome     `json:"outcome"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type AgentInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	GitCommit string `json:"git_commit,omitempty"`
}

type TaskInfo struct {
	ID      string         `json:"id" yaml:"id"`
	Input   string         `json:"input" yaml:"input"`
	Context map[string]any `json:"context,omitempty" yaml:"context,omitempty"`
}

type RunOutcome struct {
	Status     string        `json:"status"` // "completed", "failed", "timeout", "cancelled"
	Output     string        `json:"output,omitempty"`
	Error      string        `json:"error,omitempty"`
	DurationNs time.Duration `json:"duration_ns,omitempty"`
}

// Span represents an atomic step or event in an agent trajectory.
type Span struct {
	SpanID       string         `json:"span_id"`
	ParentSpanID string         `json:"parent_span_id,omitempty"`
	Name         string         `json:"name"`
	Type         SpanType       `json:"type"`
	StartTime    time.Time      `json:"start_time"`
	EndTime      time.Time      `json:"end_time"`
	Attributes   map[string]any `json:"attributes,omitempty"`
	Status       SpanStatus     `json:"status"`
}

type SpanType string

const (
	SpanTypeAgent     SpanType = "agent"
	SpanTypeLLM       SpanType = "llm"
	SpanTypeTool      SpanType = "tool"
	SpanTypeRetrieval SpanType = "retrieval"
	SpanTypeMemory    SpanType = "memory"
	SpanTypePlan      SpanType = "plan"
	SpanTypeError     SpanType = "error"
)

type SpanStatus struct {
	Code    string `json:"code"` // "ok", "error"
	Message string `json:"message,omitempty"`
}

// Fixture represents a recorded or authored external dependency response.
type Fixture struct {
	FixtureID        string           `json:"fixture_id" yaml:"fixture_id"`
	Tool             string           `json:"tool" yaml:"tool"`
	InputHash        string           `json:"input_hash,omitempty" yaml:"input_hash,omitempty"`
	MatchStrategy    MatchStrategy    `json:"match_strategy,omitempty" yaml:"match_strategy,omitempty"`
	SequenceOrder    int              `json:"sequence_order,omitempty" yaml:"sequence_order,omitempty"`
	RecordedInput    map[string]any   `json:"recorded_input,omitempty" yaml:"recorded_input,omitempty"`
	RecordedResponse RecordedResponse `json:"recorded_response" yaml:"recorded_response"`
	Mode             FailureMode      `json:"mode,omitempty" yaml:"mode,omitempty"`
	DelayMs          int              `json:"delay_ms,omitempty" yaml:"delay_ms,omitempty"`
	Provenance       ProvenanceType   `json:"provenance" yaml:"provenance"`
}

type MatchStrategy string

const (
	MatchStrategyExactHash       MatchStrategy = "exact_hash"
	MatchStrategyOrderedSequence MatchStrategy = "ordered_sequence"
	MatchStrategyHybrid          MatchStrategy = "prefer_exact_then_sequence"
)

type FailureMode string

const (
	FailureModeSuccess        FailureMode = "success"
	FailureModeTimeout        FailureMode = "timeout"
	FailureModeMalformed      FailureMode = "malformed"
	FailureModeSlow           FailureMode = "slow"
	FailureModePartialFailure FailureMode = "partial_failure"
)

type ProvenanceType string

const (
	ProvenanceRecorded ProvenanceType = "recorded"
	ProvenanceAuthored ProvenanceType = "authored"
)

type RecordedResponse struct {
	Status     string `json:"status" yaml:"status"`
	StatusCode int    `json:"status_code,omitempty" yaml:"status_code,omitempty"`
	Body       any    `json:"body" yaml:"body"`
	Error      string `json:"error,omitempty" yaml:"error,omitempty"`
}

// TestScenario is the standalone test specification artifact.
type TestScenario struct {
	ID          string                 `json:"id" yaml:"id"`
	Version     string                 `json:"version" yaml:"version"`
	Description string                 `json:"description" yaml:"description"`
	Task        TaskInfo               `json:"task" yaml:"task"`
	Environment EnvironmentSpec        `json:"environment" yaml:"environment"`
	Assertions  []Assertion            `json:"assertions" yaml:"assertions"`
	Reliability ReliabilityConfig      `json:"reliability" yaml:"reliability"`
	Provenance  TestScenarioProvenance `json:"provenance" yaml:"provenance"`
}

type EnvironmentSpec struct {
	FixtureStrategy MatchStrategy `json:"fixture_strategy,omitempty" yaml:"fixture_strategy,omitempty"`
	Fixtures        []Fixture     `json:"fixtures" yaml:"fixtures"`
}

type Assertion struct {
	ID          string           `json:"id" yaml:"id"`
	Type        AssertionType    `json:"type" yaml:"type"`
	Criticality CriticalityLevel `json:"criticality,omitempty" yaml:"criticality,omitempty"`
	Tool        string           `json:"tool,omitempty" yaml:"tool,omitempty"`
	Arguments   map[string]any   `json:"arguments,omitempty" yaml:"arguments,omitempty"`
	Limit       int              `json:"limit,omitempty" yaml:"limit,omitempty"`
	Parameters  map[string]any   `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

type CriticalityLevel string

const (
	CriticalityHard CriticalityLevel = "hard"
	CriticalitySoft CriticalityLevel = "soft"
)

type AssertionType string

const (
	AssertTaskSuccess       AssertionType = "task_success"
	AssertToolCall          AssertionType = "tool_call"
	AssertForbiddenToolCall AssertionType = "forbidden_tool_call"
	AssertRequiredTool      AssertionType = "required_tool"
	AssertToolSequence      AssertionType = "tool_sequence"
	AssertMaxSteps          AssertionType = "max_steps"
	AssertMaxLatency        AssertionType = "max_latency_ms"
	AssertSchemaValid       AssertionType = "schema_valid"
	AssertErrorRecovery     AssertionType = "error_recovery"
)

type ReliabilityConfig struct {
	Samples         int     `json:"samples" yaml:"samples"`
	MinimumPassRate float64 `json:"minimum_pass_rate" yaml:"minimum_pass_rate"`
	Confidence      float64 `json:"confidence" yaml:"confidence"`
	Deterministic   bool    `json:"deterministic,omitempty" yaml:"deterministic,omitempty"`
}

type TestScenarioProvenance struct {
	Source      string    `json:"source" yaml:"source"`
	SourceRunID string    `json:"source_run_id,omitempty" yaml:"source_run_id,omitempty"`
	ExtractedAt time.Time `json:"extracted_at" yaml:"extracted_at"`
	ReviewedBy  string    `json:"reviewed_by" yaml:"reviewed_by"`
}

// Policy defines CI and evaluation acceptance gates.
type Policy struct {
	Version         string            `json:"version" yaml:"version"`
	Name            string            `json:"name" yaml:"name"`
	HardConstraints HardConstraints   `json:"hard_constraints" yaml:"hard_constraints"`
	Reliability     PolicyReliability `json:"reliability" yaml:"reliability"`
	Regression      PolicyRegression  `json:"regression,omitempty" yaml:"regression,omitempty"`
}

type HardConstraints struct {
	ForbiddenTools   int `json:"forbidden_tools" yaml:"forbidden_tools"`
	SchemaViolations int `json:"schema_violations" yaml:"schema_violations"`
}

type PolicyReliability struct {
	DefaultMinimumPassRate float64 `json:"default_minimum_pass_rate" yaml:"default_minimum_pass_rate"`
	MinSamplesForVerdict   int     `json:"min_samples_for_verdict" yaml:"min_samples_for_verdict"`
	OnFlaky                string  `json:"on_flaky" yaml:"on_flaky"`
}

type PolicyRegression struct {
	MaxPassRateDrop         float64 `json:"max_pass_rate_drop" yaml:"max_pass_rate_drop"`
	MaxLatencyIncreaseRatio float64 `json:"max_latency_increase_ratio" yaml:"max_latency_increase_ratio"`
}

// Verdict enums
type VerdictType string

const (
	VerdictPass                VerdictType = "PASS"
	VerdictFail                VerdictType = "FAIL"
	VerdictFlaky               VerdictType = "FLAKY"
	VerdictInsufficientSamples VerdictType = "INSUFFICIENT_SAMPLES"
)

// MutationOutcome represents the typed result of applying a mutation.
type MutationStatus string

const (
	MutationApplied MutationStatus = "applied"
	MutationSkipped MutationStatus = "skipped"
	MutationError   MutationStatus = "error"
)

type MutationOutcome struct {
	Status      MutationStatus `json:"status"`
	Class       string         `json:"class"`
	OriginalRun AgentRun       `json:"original_run"`
	MutatedRun  AgentRun       `json:"mutated_run,omitempty"`
	SkipReason  string         `json:"skip_reason,omitempty"`
	Description string         `json:"description"`
}

// Validation methods for domain entities

func (r *AgentRun) Validate() error {
	if r.SchemaVersion == "" {
		return fmt.Errorf("%w: schema_version is required", ErrMissingRequiredField)
	}
	if r.SchemaVersion != SchemaVersion {
		return fmt.Errorf("%w: expected %s, got %s", ErrInvalidSchemaVersion, SchemaVersion, r.SchemaVersion)
	}
	if r.RunID == "" {
		return fmt.Errorf("%w: run_id is required", ErrMissingRequiredField)
	}
	if r.Agent.Name == "" || r.Agent.Version == "" {
		return fmt.Errorf("%w: agent name and version are required", ErrMissingRequiredField)
	}
	if r.Task.ID == "" || r.Task.Input == "" {
		return fmt.Errorf("%w: task id and input are required", ErrMissingRequiredField)
	}
	if r.Outcome.Status == "" {
		return fmt.Errorf("%w: outcome status is required", ErrMissingRequiredField)
	}
	switch r.Outcome.Status {
	case "completed", "failed", "timeout", "cancelled":
	default:
		return fmt.Errorf("%w: unknown outcome status %q", ErrInvalidFieldValue, r.Outcome.Status)
	}
	for i, sp := range r.Trace {
		if sp.SpanID == "" {
			return fmt.Errorf("%w: trace[%d].span_id is required", ErrMissingRequiredField, i)
		}
		if sp.Name == "" {
			return fmt.Errorf("%w: trace[%d].name is required", ErrMissingRequiredField, i)
		}
	}
	return nil
}

func (s *TestScenario) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("%w: scenario id is required", ErrMissingRequiredField)
	}
	if s.Task.Input == "" {
		return fmt.Errorf("%w: task input is required", ErrMissingRequiredField)
	}
	if s.Reliability.Samples <= 0 && !s.Reliability.Deterministic {
		return fmt.Errorf("%w: reliability.samples must be > 0 unless deterministic", ErrInvalidFieldValue)
	}
	if s.Reliability.MinimumPassRate < 0.0 || s.Reliability.MinimumPassRate > 1.0 {
		return fmt.Errorf("%w: minimum_pass_rate must be between 0.0 and 1.0", ErrInvalidFieldValue)
	}
	return nil
}

func (f *Fixture) Validate() error {
	if f.FixtureID == "" {
		return fmt.Errorf("%w: fixture_id is required", ErrMissingRequiredField)
	}
	if f.Tool == "" {
		return fmt.Errorf("%w: tool is required", ErrMissingRequiredField)
	}
	if f.Provenance != ProvenanceRecorded && f.Provenance != ProvenanceAuthored {
		return fmt.Errorf("%w: provenance must be 'recorded' or 'authored'", ErrInvalidFieldValue)
	}
	return nil
}

func (p *Policy) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("%w: policy name is required", ErrMissingRequiredField)
	}
	if p.Reliability.DefaultMinimumPassRate <= 0.0 || p.Reliability.DefaultMinimumPassRate > 1.0 {
		return fmt.Errorf("%w: default_minimum_pass_rate must be between 0.0 and 1.0", ErrInvalidFieldValue)
	}
	onFlaky := strings.ToLower(p.Reliability.OnFlaky)
	if onFlaky != "warn" && onFlaky != "fail" && onFlaky != "ignore" {
		return fmt.Errorf("%w: on_flaky must be 'warn', 'fail', or 'ignore'", ErrInvalidFieldValue)
	}
	return nil
}
