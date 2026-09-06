# Phase 1: Core Domain Entities, Schemas & Canonical Serialization

## 1. Objectives & Scope
1. Define all primary domain entities in Go (`AgentRun`, `Span`, `Fixture`, `TestScenario`, `Assertion`, `Policy`, `EvaluationResult`, `ReliabilityResult`, `MutationResult`).
2. Author JSON Schema Draft 2020-12 definitions in `spec/schemas/` as the single cross-language source of truth.
3. Implement RFC 8785 JSON Canonicalization Scheme (JCS) with SHA-256 for bit-for-bit deterministic content-addressed hashing across languages.
4. Provide comprehensive validation logic with rich, path-specific error messages.

---

## 2. Package Architecture

```text
gust/
├── pkg/
│   ├── api/                     # Public Go structs and schema constants
│   │   ├── types.go             # Domain entity definitions
│   │   ├── outcome.go           # Status enums and outcome types
│   │   └── validate.go          # Schema validation helper
│   └── jcs/                     # RFC 8785 JSON Canonicalization
│       ├── canonical.go         # JCS sorting, whitespace stripping, number formatting
│       ├── hash.go              # SHA-256 content addressing helper
│       └── canonical_test.go    # RFC 8785 test suite compliance
└── spec/
    └── schemas/
        ├── agent_run.json
        ├── fixture.json
        ├── test_scenario.json
        ├── policy.json
        ├── evaluation_result.json
        └── reliability_result.json
```

---

## 3. Detailed Struct Definitions (`pkg/api/types.go`)

```go
package api

import "time"

// SchemaVersion defines current spec version
const SchemaVersion = "0.5"

// AgentRun represents a recorded execution trace.
type AgentRun struct {
    SchemaVersion string            `json:"schema_version"`
    RunID         string            `json:"run_id"`
    Agent         AgentInfo         `json:"agent"`
    Task          TaskInfo          `json:"task"`
    Trace         []Span            `json:"trace"`
    Outcome       RunOutcome        `json:"outcome"`
    Metadata      map[string]any    `json:"metadata,omitempty"`
}

type AgentInfo struct {
    Name      string `json:"name"`
    Version   string `json:"version"`
    GitCommit string `json:"git_commit,omitempty"`
}

type TaskInfo struct {
    ID      string         `json:"id"`
    Input   string         `json:"input"`
    Context map[string]any `json:"context,omitempty"`
}

type RunOutcome struct {
    Status   string `json:"status"` // "completed", "failed", "timeout", "cancelled"
    Output   string `json:"output,omitempty"`
    Error    string `json:"error,omitempty"`
    Duration time.Duration `json:"duration_ns,omitempty"`
}

// Span represents an atomic unit of execution within an agent trajectory.
type Span struct {
    SpanID       string         `json:"span_id"`
    ParentSpanID string         `json:"parent_span_id,omitempty"`
    Name         string         `json:"name"`
    Type         SpanType       `json:"type"` // "agent", "llm", "tool", "retrieval", "memory", "plan", "error"
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
    FixtureID        string           `json:"fixture_id"`
    Tool             string           `json:"tool"`
    InputHash        string           `json:"input_hash,omitempty"`
    MatchStrategy    MatchStrategy    `json:"match_strategy,omitempty"` // "exact_hash", "ordered_sequence"
    SequenceOrder    int              `json:"sequence_order,omitempty"`
    RecordedInput    map[string]any   `json:"recorded_input,omitempty"`
    RecordedResponse RecordedResponse `json:"recorded_response"`
    Mode             FailureMode      `json:"mode,omitempty"`           // "success", "timeout", "malformed", "slow", "partial_failure"
    DelayMs          int              `json:"delay_ms,omitempty"`
    Provenance       ProvenanceType   `json:"provenance"`               // "recorded", "authored"
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
    Status     string `json:"status"` // "success", "error"
    StatusCode int    `json:"status_code,omitempty"`
    Body       any    `json:"body"`
    Error      string `json:"error,omitempty"`
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
    ID          string            `json:"id" yaml:"id"`
    Type        AssertionType     `json:"type" yaml:"type"`
    Criticality CriticalityLevel  `json:"criticality,omitempty" yaml:"criticality,omitempty"` // "hard", "soft"
    Tool        string            `json:"tool,omitempty" yaml:"tool,omitempty"`
    Arguments   map[string]any    `json:"arguments,omitempty" yaml:"arguments,omitempty"`
    Limit       int               `json:"limit,omitempty" yaml:"limit,omitempty"`
    Parameters  map[string]any    `json:"parameters,omitempty" yaml:"parameters,omitempty"`
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
```

---

## 4. RFC 8785 Canonical JSON & Content Addressing (`pkg/jcs/`)

To prevent differences in whitespace, key ordering, or floating point representation between Go, Python, and CLI:
1. `Canonicalize(data []byte) ([]byte, error)`:
   - Recursively sorts dictionary keys by UTF-16 code units.
   - Strips all insignificant whitespace.
   - Formats numbers per ECMAScript specifications (no trailing `.0`, exponential notation for extreme magnitudes).
   - Validates UTF-8 encoding.
2. `ContentHash(v any) (string, error)`:
   - Serializes object to JSON.
   - Canonicalizes bytes via JCS.
   - Computes SHA-256.
   - Returns format `sha256:<64-char-hex>`.

---

## 5. Schema Validation & Error Reporting

1. Embedded JSON Schemas using `embed.FS` in `pkg/api/schemas`.
2. Validation via `santhosh-tekuri/jsonschema/v5`.
3. Clear error formatting:
   ```text
   Schema validation failed for TestScenario:
     - /assertions/0/type: must be one of ["task_success", "tool_call", ...]
     - /reliability/samples: must be greater than or equal to 1
   ```

---

## 6. Verification & Test Plan

- **Unit Tests (`pkg/jcs/canonical_test.go`):**
  - Run the official RFC 8785 test suite JSON vectors.
  - Verify key sorting with unicode characters.
  - Verify float formatting (`1.0` -> `1`, `1e20` format).
- **Round-Trip Serialization Tests (`pkg/api/roundtrip_test.go`):**
  - Unmarshal JSON -> Go Struct -> JCS Marshal -> SHA-256 -> Compare with golden vector.
- **Validation Tests (`pkg/api/validate_test.go`):**
  - Verify valid files pass without error.
  - Verify invalid fields produce descriptive JSON path error messages.
