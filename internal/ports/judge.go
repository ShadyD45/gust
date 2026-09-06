package ports

import (
	"context"

	"gust/pkg/api"
)

// JudgeRequest is the input to an optional LLM judge.
type JudgeRequest struct {
	Run        api.AgentRun
	Rubric     string
	Threshold  float64 // pass if score >= threshold; default 0.7
	Model      string
	FewShot    string
	Parameters map[string]any
}

// JudgeResult is the judge's scored opinion of a run.
type JudgeResult struct {
	Score     float64 `json:"score"` // 0.0–1.0
	Passed    bool    `json:"passed"`
	Rationale string  `json:"rationale,omitempty"`
	Model     string  `json:"model,omitempty"`
	Raw       string  `json:"raw,omitempty"`
}

// JudgeProvider scores open-ended / subjective aspects of an AgentRun.
// Deterministic evaluators remain the default CI path; judges are opt-in soft signals.
type JudgeProvider interface {
	Name() string
	Judge(ctx context.Context, req JudgeRequest) (JudgeResult, error)
}
