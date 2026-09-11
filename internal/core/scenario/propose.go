package scenario

import (
	"fmt"
	"time"

	"gust/internal/core/redact"
	"gust/pkg/api"
)

// Proposal is a reviewable TestScenario draft plus redaction audit metadata.
type Proposal struct {
	Scenario       *api.TestScenario `json:"scenario"`
	Redaction      redact.Result     `json:"redaction"`
	ProposedAt     time.Time         `json:"proposed_at"`
	AssertionsNote string            `json:"assertions_note"`
}

// ProposeOptions controls continuous-eval proposal generation.
type ProposeOptions struct {
	Redact          bool
	RedactOptOutNote string
	Rules           []redact.Rule
	Source          string // default "continuous_eval"
}

// ProposeFromRun redacts (by default), extracts a scenario with empty assertions,
// and leaves reviewed_by empty until a human reviews it.
func (e *Extractor) ProposeFromRun(run api.AgentRun, opts ProposeOptions) (*Proposal, error) {
	if opts.Source == "" {
		opts.Source = "continuous_eval"
	}
	cfg := redact.Config{
		Enabled:    opts.Redact,
		Rules:      opts.Rules,
		OptOutNote: opts.RedactOptOutNote,
	}
	if !opts.Redact && opts.RedactOptOutNote == "" {
		return nil, fmt.Errorf("redaction opt-out requires --redact-opt-out-note for audit")
	}
	scrubbed, redRes, err := redact.RedactRun(run, cfg)
	if err != nil {
		return nil, err
	}
	sc, err := e.ExtractFromRun(scrubbed)
	if err != nil {
		return nil, err
	}
	sc.Provenance.Source = opts.Source
	sc.Provenance.SourceRunID = run.RunID
	sc.Provenance.ExtractedAt = time.Now().UTC()
	sc.Provenance.ReviewedBy = "" // mandatory empty until human review
	sc.Assertions = nil           // never auto-fill from behavior
	if sc.Assertions == nil {
		sc.Assertions = []api.Assertion{}
	}
	return &Proposal{
		Scenario:       sc,
		Redaction:      redRes,
		ProposedAt:     time.Now().UTC(),
		AssertionsNote: "assertions left empty — human must author expectations (H8)",
	}, nil
}
