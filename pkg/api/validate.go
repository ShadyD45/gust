package api

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	sha256Regex = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
)

// ValidateJSON validates that raw JSON unmarshals cleanly and satisfies domain constraints.
func ValidateJSON[T interface{ Validate() error }](data []byte, target T) error {
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("json unmarshal failed: %w", err)
	}
	return target.Validate()
}

// DeepValidation helpers

func ValidateSpan(sp *Span, idx int) error {
	if strings.TrimSpace(sp.SpanID) == "" {
		return fmt.Errorf("trace[%d].span_id must not be empty", idx)
	}
	if strings.TrimSpace(sp.Name) == "" {
		return fmt.Errorf("trace[%d].name must not be empty", idx)
	}
	switch sp.Type {
	case SpanTypeAgent, SpanTypeLLM, SpanTypeTool, SpanTypeRetrieval, SpanTypeMemory, SpanTypePlan, SpanTypeError:
	default:
		return fmt.Errorf("trace[%d].type %q is invalid", idx, sp.Type)
	}
	if sp.StartTime.IsZero() {
		return fmt.Errorf("trace[%d].start_time must be set", idx)
	}
	if sp.EndTime.IsZero() {
		return fmt.Errorf("trace[%d].end_time must be set", idx)
	}
	if sp.EndTime.Before(sp.StartTime) {
		return fmt.Errorf("trace[%d].end_time cannot be before start_time", idx)
	}
	if sp.Status.Code != "ok" && sp.Status.Code != "error" {
		return fmt.Errorf("trace[%d].status.code must be 'ok' or 'error', got %q", idx, sp.Status.Code)
	}
	return nil
}

func ValidateAssertion(a *Assertion, idx int) error {
	if strings.TrimSpace(a.ID) == "" {
		return fmt.Errorf("assertions[%d].id is required", idx)
	}
	switch a.Type {
	case AssertTaskSuccess, AssertToolCall, AssertForbiddenToolCall, AssertRequiredTool,
		AssertToolSequence, AssertMaxSteps, AssertMaxLatency, AssertSchemaValid, AssertErrorRecovery:
	default:
		return fmt.Errorf("assertions[%d].type %q is invalid", idx, a.Type)
	}
	if a.Criticality != "" && a.Criticality != CriticalityHard && a.Criticality != CriticalitySoft {
		return fmt.Errorf("assertions[%d].criticality must be 'hard' or 'soft', got %q", idx, a.Criticality)
	}
	return nil
}

func ValidateFixtureObject(f *Fixture) error {
	if strings.TrimSpace(f.FixtureID) == "" {
		return fmt.Errorf("fixture_id is required")
	}
	if strings.TrimSpace(f.Tool) == "" {
		return fmt.Errorf("tool is required")
	}
	if f.InputHash != "" && !sha256Regex.MatchString(f.InputHash) {
		return fmt.Errorf("input_hash must match pattern ^sha256:[a-f0-9]{64}$, got %q", f.InputHash)
	}
	if f.MatchStrategy != "" {
		switch f.MatchStrategy {
		case MatchStrategyExactHash, MatchStrategyOrderedSequence, MatchStrategyHybrid:
		default:
			return fmt.Errorf("match_strategy %q is invalid", f.MatchStrategy)
		}
	}
	if f.Mode != "" {
		switch f.Mode {
		case FailureModeSuccess, FailureModeTimeout, FailureModeMalformed, FailureModeSlow, FailureModePartialFailure:
		default:
			return fmt.Errorf("mode %q is invalid", f.Mode)
		}
	}
	if f.Provenance != ProvenanceRecorded && f.Provenance != ProvenanceAuthored {
		return fmt.Errorf("provenance must be 'recorded' or 'authored', got %q", f.Provenance)
	}
	return nil
}

// CheckDatesValid checks date-time bounds
func CheckDatesValid(t time.Time) bool {
	return !t.IsZero()
}
