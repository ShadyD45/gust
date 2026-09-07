package validation

import (
	"gust/internal/ports"
	"gust/pkg/api"
)

// Kind selects how a case is executed.
type Kind string

const (
	KindAnalyze  Kind = "analyze"
	KindWilson   Kind = "wilson"
	KindInfra    Kind = "infra"
	KindMutation Kind = "mutation"
	KindFixture  Kind = "fixture"
)

// Category groups cases by the review's trust questions.
type Category string

const (
	CatPass     Category = "pass"
	CatFail     Category = "fail"
	CatFlaky    Category = "flaky"
	CatInfra    Category = "infra"
	CatMutation Category = "mutation"
	CatFixture  Category = "fixture"
	CatRecovery Category = "recovery"
)

// Case is one adversarial scenario with a known expected outcome.
type Case struct {
	ID        string   `json:"id"`
	Category  Category `json:"category"`
	Question  string   `json:"question"`
	Rationale string   `json:"rationale"`
	Kind      Kind     `json:"kind"`

	// KindAnalyze / recovery-style analyze cases.
	Run        api.AgentRun    `json:"-"`
	Assertions []api.Assertion `json:"-"`
	WantPassed bool            `json:"want_passed,omitempty"`

	// KindWilson
	Passes     int     `json:"passes,omitempty"`
	Total      int     `json:"total,omitempty"`
	MinPass    float64 `json:"min_pass,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	MinSamples int     `json:"min_samples,omitempty"`
	WantVerdict api.VerdictType `json:"want_verdict,omitempty"`

	// KindInfra
	FailFirstN  int     `json:"fail_first_n,omitempty"`
	Samples     int     `json:"samples,omitempty"`
	MaxExecRate float64 `json:"max_exec_rate,omitempty"`
	WantUnstable bool   `json:"want_unstable,omitempty"`
	WantExecErrs int    `json:"want_exec_errs,omitempty"`

	// KindMutation
	MutatorName string `json:"mutator,omitempty"`
	GoldenIdx   int    `json:"golden_idx,omitempty"`
	WantDetected bool  `json:"want_detected,omitempty"`

	// KindFixture
	Fixtures     []api.Fixture    `json:"-"`
	Call         ports.ToolCall   `json:"-"`
	WantFound    bool             `json:"want_found,omitempty"`
	WantStatus   string           `json:"want_status,omitempty"`
	CallSequence []ports.ToolCall `json:"-"` // ordered lookups; last result checked if set
	WantBodies   []string         `json:"-"` // expected Body string for each sequential call
}

// CaseResult is the outcome of running one case.
type CaseResult struct {
	ID       string   `json:"id"`
	Category Category `json:"category"`
	Question string   `json:"question"`
	Kind     Kind     `json:"kind"`
	Passed   bool     `json:"passed"`
	Detail   string   `json:"detail,omitempty"`
}

// Report summarizes the Gust Validation Suite run.
type Report struct {
	GeneratedAt   string                `json:"generated_at"`
	SuiteVersion  string                `json:"suite_version"`
	Total         int                   `json:"total"`
	Passed        int                   `json:"passed"`
	Failed        int                   `json:"failed"`
	PassRate      float64               `json:"pass_rate"`
	GatePassed    bool                  `json:"gate_passed"`
	ByCategory    map[string]CatStats   `json:"by_category"`
	Failures      []CaseResult          `json:"failures,omitempty"`
	Results       []CaseResult          `json:"results"`
}

// CatStats is per-category counts.
type CatStats struct {
	Total  int     `json:"total"`
	Passed int     `json:"passed"`
	Rate   float64 `json:"rate"`
}

// SuiteVersion bumps when case semantics change in a breaking way.
const SuiteVersion = "1.0.0"

// MinPassRate is the gate for publishing trust: every case must pass.
const MinPassRate = 1.0
