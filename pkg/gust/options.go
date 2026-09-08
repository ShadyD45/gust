package gust

import (
	"context"

	"gust/internal/adapters/evaluators"
	"gust/internal/ports"
	"gust/pkg/api"
)

type options struct {
	evaluators []Evaluator
	evalCtx    EvaluationContext
}

// Option configures Analyze / NewAnalyzer.
type Option func(*options)

// WithEvaluators replaces the default built-in evaluator set.
// Pass BuiltinEvaluators() plus your own to extend the defaults.
func WithEvaluators(evs ...Evaluator) Option {
	return func(o *options) {
		o.evaluators = append([]Evaluator(nil), evs...)
	}
}

// WithExtraEvaluators appends custom evaluators to the built-in set.
func WithExtraEvaluators(evs ...Evaluator) Option {
	return func(o *options) {
		if o.evaluators == nil {
			o.evaluators = BuiltinEvaluators()
		}
		o.evaluators = append(o.evaluators, evs...)
	}
}

// WithScenarioID sets EvaluationContext.ScenarioID for the analyze call.
func WithScenarioID(id string) Option {
	return func(o *options) { o.evalCtx.ScenarioID = id }
}

// WithEvaluationContext replaces the evaluation context passed to evaluators.
func WithEvaluationContext(ctx EvaluationContext) Option {
	return func(o *options) { o.evalCtx = ctx }
}

func applyOptions(opts []Option) options {
	o := options{
		evaluators: BuiltinEvaluators(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	if o.evaluators == nil {
		o.evaluators = BuiltinEvaluators()
	}
	return o
}

// BuiltinEvaluators returns the same deterministic evaluators the gust CLI uses.
func BuiltinEvaluators() []Evaluator {
	in := evaluators.AllBuiltinEvaluators()
	out := make([]Evaluator, 0, len(in))
	for _, e := range in {
		out = append(out, wrapPortEvaluator(e))
	}
	return out
}

type portEvaluatorAdapter struct {
	inner ports.Evaluator
}

func wrapPortEvaluator(e ports.Evaluator) Evaluator {
	return portEvaluatorAdapter{inner: e}
}

func (a portEvaluatorAdapter) Name() string    { return a.inner.Name() }
func (a portEvaluatorAdapter) Version() string { return a.inner.Version() }

func (a portEvaluatorAdapter) Evaluate(
	ctx context.Context,
	run api.AgentRun,
	expected *api.Assertion,
	evalCtx EvaluationContext,
) (api.EvaluationResult, error) {
	res, err := a.inner.Evaluate(ctx, run, expected, toPortEvalCtx(evalCtx))
	if err != nil {
		return api.EvaluationResult{}, err
	}
	return toAPIResult(res), nil
}

type publicEvaluatorAdapter struct {
	inner Evaluator
}

func toPortEvaluators(evs []Evaluator) []ports.Evaluator {
	out := make([]ports.Evaluator, 0, len(evs))
	for _, e := range evs {
		out = append(out, publicEvaluatorAdapter{inner: e})
	}
	return out
}

func (a publicEvaluatorAdapter) Name() string    { return a.inner.Name() }
func (a publicEvaluatorAdapter) Version() string { return a.inner.Version() }

func (a publicEvaluatorAdapter) Evaluate(
	ctx context.Context,
	run api.AgentRun,
	expected *api.Assertion,
	evalCtx ports.EvaluationContext,
) (ports.EvaluationResult, error) {
	res, err := a.inner.Evaluate(ctx, run, expected, fromPortEvalCtx(evalCtx))
	if err != nil {
		return ports.EvaluationResult{}, err
	}
	return toPortResult(res), nil
}

func toPortEvalCtx(c EvaluationContext) ports.EvaluationContext {
	return ports.EvaluationContext{
		ScenarioID:  c.ScenarioID,
		Environment: c.Environment,
		Config:      c.Config,
	}
}

func fromPortEvalCtx(c ports.EvaluationContext) EvaluationContext {
	return EvaluationContext{
		ScenarioID:  c.ScenarioID,
		Environment: c.Environment,
		Config:      c.Config,
	}
}

func toAPIResult(r ports.EvaluationResult) api.EvaluationResult {
	return api.EvaluationResult{
		EvaluatorName:    r.EvaluatorName,
		EvaluatorVersion: r.EvaluatorVersion,
		Passed:           r.Passed,
		Score:            r.Score,
		Message:          r.Message,
		Evidence:         r.Evidence,
		ExecutionTimeNs:  r.ExecutionTimeNs,
		Criticality:      r.Criticality,
	}
}

func toPortResult(r api.EvaluationResult) ports.EvaluationResult {
	return ports.EvaluationResult{
		EvaluatorName:    r.EvaluatorName,
		EvaluatorVersion: r.EvaluatorVersion,
		Passed:           r.Passed,
		Score:            r.Score,
		Message:          r.Message,
		Evidence:         r.Evidence,
		ExecutionTimeNs:  r.ExecutionTimeNs,
		Criticality:      r.Criticality,
	}
}
