package evaluators

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gust/internal/domain/evidence"
	"gust/internal/ports"
	"gust/internal/registry"
	"gust/pkg/api"
)

// --- 1. TaskSuccessEvaluator ---
type TaskSuccessEvaluator struct{}

func (e *TaskSuccessEvaluator) Name() string    { return "task_success" }
func (e *TaskSuccessEvaluator) Version() string { return "1.0.0" }
func (e *TaskSuccessEvaluator) Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx ports.EvaluationContext) (ports.EvaluationResult, error) {
	start := time.Now()
	res := ports.EvaluationResult{
		EvaluatorName:    e.Name(),
		EvaluatorVersion: e.Version(),
	}

	if run.Outcome.Status != "completed" {
		res.Passed = false
		res.Score = 0.0
		res.Message = fmt.Sprintf("task failed with outcome status %q", run.Outcome.Status)
		res.Evidence = map[string]any{
			"status": run.Outcome.Status,
			"error":  run.Outcome.Error,
		}
		res.ExecutionTimeNs = time.Since(start).Nanoseconds()
		return res, nil
	}

	// If expected output is declared in assertion parameters
	if expected != nil && expected.Parameters != nil {
		if expOut, ok := expected.Parameters["expected_output"].(string); ok && expOut != "" {
			if !strings.Contains(run.Outcome.Output, expOut) {
				res.Passed = false
				res.Score = 0.5
				res.Message = fmt.Sprintf("output %q does not contain expected %q", run.Outcome.Output, expOut)
				res.Evidence = map[string]any{
					"actual_output":   run.Outcome.Output,
					"expected_output": expOut,
				}
				res.ExecutionTimeNs = time.Since(start).Nanoseconds()
				return res, nil
			}
		}
	}

	res.Passed = true
	res.Score = 1.0
	res.Message = "task completed successfully"
	res.ExecutionTimeNs = time.Since(start).Nanoseconds()
	return res, nil
}

// --- 2. ToolSelectionEvaluator ---
type ToolSelectionEvaluator struct{}

func (e *ToolSelectionEvaluator) Name() string    { return "tool_selection" }
func (e *ToolSelectionEvaluator) Version() string { return "1.0.0" }
func (e *ToolSelectionEvaluator) Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx ports.EvaluationContext) (ports.EvaluationResult, error) {
	start := time.Now()
	res := ports.EvaluationResult{
		EvaluatorName:    e.Name(),
		EvaluatorVersion: e.Version(),
	}

	if expected == nil || expected.Tool == "" {
		res.Passed = true
		res.Score = 1.0
		return res, nil
	}

	found := false
	var calledTools []string
	for _, sp := range run.Trace {
		if sp.Type == api.SpanTypeTool {
			calledTools = append(calledTools, sp.Name)
			if sp.Name == expected.Tool {
				found = true
			}
		}
	}

	if !found {
		res.Passed = false
		res.Score = 0.0
		res.Message = fmt.Sprintf("expected tool %q was not called", expected.Tool)
		res.Evidence = map[string]any{
			"expected_tool": expected.Tool,
			"called_tools":  calledTools,
		}
		res.ExecutionTimeNs = time.Since(start).Nanoseconds()
		return res, nil
	}

	res.Passed = true
	res.Score = 1.0
	res.Message = fmt.Sprintf("tool %q was successfully selected", expected.Tool)
	res.ExecutionTimeNs = time.Since(start).Nanoseconds()
	return res, nil
}

// --- 3. ToolArgumentsEvaluator ---
type ToolArgumentsEvaluator struct{}

func (e *ToolArgumentsEvaluator) Name() string    { return "tool_arguments" }
func (e *ToolArgumentsEvaluator) Version() string { return "1.0.0" }
func (e *ToolArgumentsEvaluator) Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx ports.EvaluationContext) (ports.EvaluationResult, error) {
	start := time.Now()
	res := ports.EvaluationResult{
		EvaluatorName:    e.Name(),
		EvaluatorVersion: e.Version(),
	}

	if expected == nil || len(expected.Arguments) == 0 {
		res.Passed = true
		res.Score = 1.0
		return res, nil
	}

	var matchedToolSpans []api.Span
	for _, sp := range run.Trace {
		if sp.Type == api.SpanTypeTool && (expected.Tool == "" || sp.Name == expected.Tool) {
			matchedToolSpans = append(matchedToolSpans, sp)
		}
	}

	if len(matchedToolSpans) == 0 {
		res.Passed = false
		res.Score = 0.0
		res.Message = fmt.Sprintf("no tool spans found for %q", expected.Tool)
		res.ExecutionTimeNs = time.Since(start).Nanoseconds()
		return res, nil
	}

	// Check if any invocation had matching arguments
	var allDiffs []any
	for _, sp := range matchedToolSpans {
		actArgs, _ := sp.Attributes["input"].(map[string]any)
		diffs := evidence.CompareMaps(expected.Arguments, actArgs, "")
		if len(diffs) == 0 {
			res.Passed = true
			res.Score = 1.0
			res.Message = "tool arguments matched expected specifications"
			res.ExecutionTimeNs = time.Since(start).Nanoseconds()
			return res, nil
		}
		allDiffs = append(allDiffs, map[string]any{
			"span_id": sp.SpanID,
			"diffs":   diffs,
		})
	}

	res.Passed = false
	res.Score = 0.0
	res.Message = "tool arguments did not match expected values"
	res.Evidence = map[string]any{
		"expected_arguments": expected.Arguments,
		"discrepancies":      allDiffs,
	}
	res.ExecutionTimeNs = time.Since(start).Nanoseconds()
	return res, nil
}

// --- 4. ToolSequenceEvaluator ---
type ToolSequenceEvaluator struct{}

func (e *ToolSequenceEvaluator) Name() string    { return "tool_sequence" }
func (e *ToolSequenceEvaluator) Version() string { return "1.0.0" }
func (e *ToolSequenceEvaluator) Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx ports.EvaluationContext) (ports.EvaluationResult, error) {
	start := time.Now()
	res := ports.EvaluationResult{
		EvaluatorName:    e.Name(),
		EvaluatorVersion: e.Version(),
	}

	if expected == nil || expected.Parameters == nil {
		res.Passed = true
		res.Score = 1.0
		return res, nil
	}

	seqRaw, ok := expected.Parameters["sequence"].([]any)
	if !ok || len(seqRaw) == 0 {
		res.Passed = true
		res.Score = 1.0
		return res, nil
	}

	var expectedSeq []string
	for _, s := range seqRaw {
		if str, ok := s.(string); ok {
			expectedSeq = append(expectedSeq, str)
		}
	}

	var actualSeq []string
	for _, sp := range run.Trace {
		if sp.Type == api.SpanTypeTool {
			actualSeq = append(actualSeq, sp.Name)
		}
	}

	// Verify expected sequence appears in order within actualSeq
	seqIdx := 0
	for _, act := range actualSeq {
		if seqIdx < len(expectedSeq) && act == expectedSeq[seqIdx] {
			seqIdx++
		}
	}

	if seqIdx < len(expectedSeq) {
		res.Passed = false
		res.Score = float64(seqIdx) / float64(len(expectedSeq))
		res.Message = fmt.Sprintf("expected sequence %v was broken; only matched up to %v", expectedSeq, expectedSeq[:seqIdx])
		res.Evidence = map[string]any{
			"expected_sequence": expectedSeq,
			"actual_sequence":   actualSeq,
		}
		res.ExecutionTimeNs = time.Since(start).Nanoseconds()
		return res, nil
	}

	res.Passed = true
	res.Score = 1.0
	res.Message = "tool sequence satisfied"
	res.ExecutionTimeNs = time.Since(start).Nanoseconds()
	return res, nil
}

// --- 5. ForbiddenToolEvaluator ---
type ForbiddenToolEvaluator struct{}

func (e *ForbiddenToolEvaluator) Name() string    { return "forbidden_tool" }
func (e *ForbiddenToolEvaluator) Version() string { return "1.0.0" }
func (e *ForbiddenToolEvaluator) Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx ports.EvaluationContext) (ports.EvaluationResult, error) {
	start := time.Now()
	res := ports.EvaluationResult{
		EvaluatorName:    e.Name(),
		EvaluatorVersion: e.Version(),
	}

	if expected == nil || expected.Tool == "" {
		res.Passed = true
		res.Score = 1.0
		return res, nil
	}

	for _, sp := range run.Trace {
		if sp.Type == api.SpanTypeTool && sp.Name == expected.Tool {
			// If arguments specified, check argument match
			if len(expected.Arguments) > 0 {
				actArgs, _ := sp.Attributes["input"].(map[string]any)
				diffs := evidence.CompareMaps(expected.Arguments, actArgs, "")
				if len(diffs) > 0 {
					continue // Arguments didn't match forbidden pattern
				}
			}

			res.Passed = false
			res.Score = 0.0
			res.Message = fmt.Sprintf("forbidden tool %q was called", expected.Tool)
			res.Evidence = map[string]any{
				"forbidden_tool": expected.Tool,
				"span_id":        sp.SpanID,
				"timestamp":      sp.StartTime,
				"arguments":      sp.Attributes["input"],
			}
			res.ExecutionTimeNs = time.Since(start).Nanoseconds()
			return res, nil
		}
	}

	res.Passed = true
	res.Score = 1.0
	res.Message = fmt.Sprintf("no forbidden tool %q was called", expected.Tool)
	res.ExecutionTimeNs = time.Since(start).Nanoseconds()
	return res, nil
}

// --- 6. RequiredToolEvaluator ---
type RequiredToolEvaluator struct{}

func (e *RequiredToolEvaluator) Name() string    { return "required_tool" }
func (e *RequiredToolEvaluator) Version() string { return "1.0.0" }
func (e *RequiredToolEvaluator) Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx ports.EvaluationContext) (ports.EvaluationResult, error) {
	start := time.Now()
	res := ports.EvaluationResult{
		EvaluatorName:    e.Name(),
		EvaluatorVersion: e.Version(),
	}

	if expected == nil || expected.Tool == "" {
		res.Passed = true
		res.Score = 1.0
		return res, nil
	}

	count := 0
	for _, sp := range run.Trace {
		if sp.Type == api.SpanTypeTool && sp.Name == expected.Tool {
			count++
		}
	}

	if count == 0 {
		res.Passed = false
		res.Score = 0.0
		res.Message = fmt.Sprintf("required tool %q was never called", expected.Tool)
		res.Evidence = map[string]any{
			"required_tool": expected.Tool,
		}
		res.ExecutionTimeNs = time.Since(start).Nanoseconds()
		return res, nil
	}

	res.Passed = true
	res.Score = 1.0
	res.Message = fmt.Sprintf("required tool %q called %d time(s)", expected.Tool, count)
	res.ExecutionTimeNs = time.Since(start).Nanoseconds()
	return res, nil
}

// --- 7. MaxStepsEvaluator ---
type MaxStepsEvaluator struct{}

func (e *MaxStepsEvaluator) Name() string    { return "max_steps" }
func (e *MaxStepsEvaluator) Version() string { return "1.0.0" }
func (e *MaxStepsEvaluator) Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx ports.EvaluationContext) (ports.EvaluationResult, error) {
	start := time.Now()
	res := ports.EvaluationResult{
		EvaluatorName:    e.Name(),
		EvaluatorVersion: e.Version(),
	}

	limit := 10 // Default step budget
	if expected != nil && expected.Limit > 0 {
		limit = expected.Limit
	}

	steps := len(run.Trace)
	if steps > limit {
		res.Passed = false
		res.Score = float64(limit) / float64(steps)
		res.Message = fmt.Sprintf("execution took %d steps, exceeding budget of %d", steps, limit)
		res.Evidence = map[string]any{
			"actual_steps": steps,
			"step_limit":   limit,
		}
		res.ExecutionTimeNs = time.Since(start).Nanoseconds()
		return res, nil
	}

	res.Passed = true
	res.Score = 1.0
	res.Message = fmt.Sprintf("step count %d within budget of %d", steps, limit)
	res.ExecutionTimeNs = time.Since(start).Nanoseconds()
	return res, nil
}

// --- 8. MaxLatencyEvaluator ---
type MaxLatencyEvaluator struct{}

func (e *MaxLatencyEvaluator) Name() string    { return "max_latency" }
func (e *MaxLatencyEvaluator) Version() string { return "1.0.0" }
func (e *MaxLatencyEvaluator) Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx ports.EvaluationContext) (ports.EvaluationResult, error) {
	start := time.Now()
	res := ports.EvaluationResult{
		EvaluatorName:    e.Name(),
		EvaluatorVersion: e.Version(),
	}

	limitMs := 5000 // Default 5s
	if expected != nil && expected.Limit > 0 {
		limitMs = expected.Limit
	}

	var duration time.Duration
	if len(run.Trace) > 0 {
		duration = run.Trace[len(run.Trace)-1].EndTime.Sub(run.Trace[0].StartTime)
	}

	if duration.Milliseconds() > int64(limitMs) {
		res.Passed = false
		res.Score = float64(limitMs) / float64(duration.Milliseconds())
		res.Message = fmt.Sprintf("latency %d ms exceeded limit of %d ms", duration.Milliseconds(), limitMs)
		res.Evidence = map[string]any{
			"actual_latency_ms": duration.Milliseconds(),
			"max_latency_ms":    limitMs,
		}
		res.ExecutionTimeNs = time.Since(start).Nanoseconds()
		return res, nil
	}

	res.Passed = true
	res.Score = 1.0
	res.Message = fmt.Sprintf("latency %d ms within limit", duration.Milliseconds())
	res.ExecutionTimeNs = time.Since(start).Nanoseconds()
	return res, nil
}

// --- 9. ErrorRecoveryEvaluator ---
type ErrorRecoveryEvaluator struct{}

func (e *ErrorRecoveryEvaluator) Name() string    { return "error_recovery" }
func (e *ErrorRecoveryEvaluator) Version() string { return "1.0.0" }
func (e *ErrorRecoveryEvaluator) Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx ports.EvaluationContext) (ports.EvaluationResult, error) {
	start := time.Now()
	res := ports.EvaluationResult{
		EvaluatorName:    e.Name(),
		EvaluatorVersion: e.Version(),
	}

	hasErrorSpan := false
	hasSubsequentRecovery := false

	for i, sp := range run.Trace {
		if sp.Type == api.SpanTypeError || sp.Status.Code == "error" {
			hasErrorSpan = true
			// If there are subsequent steps and overall run completed
			if i < len(run.Trace)-1 && run.Outcome.Status == "completed" {
				hasSubsequentRecovery = true
			}
		}
	}

	if !hasErrorSpan {
		res.Passed = true
		res.Score = 1.0
		res.Message = "no errors occurred during trajectory"
		res.ExecutionTimeNs = time.Since(start).Nanoseconds()
		return res, nil
	}

	if !hasSubsequentRecovery {
		res.Passed = false
		res.Score = 0.0
		res.Message = "agent encountered error and failed to recover"
		res.Evidence = map[string]any{
			"error_occurred": true,
			"outcome_status": run.Outcome.Status,
		}
		res.ExecutionTimeNs = time.Since(start).Nanoseconds()
		return res, nil
	}

	res.Passed = true
	res.Score = 1.0
	res.Message = "agent successfully recovered from error"
	res.ExecutionTimeNs = time.Since(start).Nanoseconds()
	return res, nil
}

// --- 10. SchemaValidationEvaluator ---
type SchemaValidationEvaluator struct{}

func (e *SchemaValidationEvaluator) Name() string    { return "schema_validation" }
func (e *SchemaValidationEvaluator) Version() string { return "1.0.0" }
func (e *SchemaValidationEvaluator) Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx ports.EvaluationContext) (ports.EvaluationResult, error) {
	start := time.Now()
	res := ports.EvaluationResult{
		EvaluatorName:    e.Name(),
		EvaluatorVersion: e.Version(),
	}

	if err := run.Validate(); err != nil {
		res.Passed = false
		res.Score = 0.0
		res.Message = fmt.Sprintf("run failed schema validation: %v", err)
		res.Evidence = map[string]any{"validation_error": err.Error()}
		res.ExecutionTimeNs = time.Since(start).Nanoseconds()
		return res, nil
	}

	res.Passed = true
	res.Score = 1.0
	res.Message = "run satisfies schema validation"
	res.ExecutionTimeNs = time.Since(start).Nanoseconds()
	return res, nil
}

// RegisterBuiltinEvaluators registers all built-in evaluators with the registry.
func RegisterBuiltinEvaluators(r *registry.Registry) {
	for _, e := range AllBuiltinEvaluators() {
		_ = r.RegisterEvaluator(e)
	}
}

// AllBuiltinEvaluators returns the complete list of built-in evaluators
// (10 deterministic MVP evaluators plus the opt-in llm_judge adapter).
func AllBuiltinEvaluators() []ports.Evaluator {
	return []ports.Evaluator{
		&TaskSuccessEvaluator{},
		&ToolSelectionEvaluator{},
		&ToolArgumentsEvaluator{},
		&ToolSequenceEvaluator{},
		&ForbiddenToolEvaluator{},
		&RequiredToolEvaluator{},
		&MaxStepsEvaluator{},
		&MaxLatencyEvaluator{},
		&ErrorRecoveryEvaluator{},
		&SchemaValidationEvaluator{},
		NewLLMJudgeEvaluator(nil),
	}
}
