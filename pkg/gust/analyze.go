package gust

import (
	"context"

	"gust/internal/core/analyze"
	"gust/pkg/api"
)

// Analyzer runs Mode 1 (Analyze) against captured AgentRun documents.
type Analyzer struct {
	opts options
}

// NewAnalyzer constructs an Analyzer with the given options.
// By default it uses BuiltinEvaluators().
func NewAnalyzer(opts ...Option) *Analyzer {
	return &Analyzer{opts: applyOptions(opts)}
}

// Analyze evaluates assertions against a captured run using this analyzer's
// evaluator set and context defaults.
func (a *Analyzer) Analyze(ctx context.Context, run api.AgentRun, assertions []api.Assertion) (*AnalysisReport, error) {
	return analyzeWith(ctx, run, assertions, a.opts)
}

// Analyze evaluates a captured AgentRun against assertions using the built-in
// evaluators (unless overridden with options).
//
// This is the supported library entrypoint for Mode 1. Do not import
// gust/internal/core/analyze from application code.
func Analyze(ctx context.Context, run api.AgentRun, assertions []api.Assertion, opts ...Option) (*AnalysisReport, error) {
	return analyzeWith(ctx, run, assertions, applyOptions(opts))
}

func analyzeWith(ctx context.Context, run api.AgentRun, assertions []api.Assertion, opts options) (*AnalysisReport, error) {
	engine := analyze.NewEngine(toPortEvaluators(opts.evaluators))
	report, err := engine.AnalyzeRun(ctx, run, assertions, toPortEvalCtx(opts.evalCtx))
	if err != nil {
		return nil, err
	}
	out := &AnalysisReport{
		RunID:           report.RunID,
		Passed:          report.Passed,
		TotalDurationNs: report.TotalDurationNs,
		Results:         make([]api.EvaluationResult, 0, len(report.Results)),
	}
	for _, r := range report.Results {
		out.Results = append(out.Results, toAPIResult(r))
	}
	return out, nil
}
