package mutators

import (
	"context"
	"fmt"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

// Helper to clone an AgentRun
func cloneRun(run api.AgentRun) api.AgentRun {
	cloned := run
	cloned.Trace = make([]api.Span, len(run.Trace))
	for i, sp := range run.Trace {
		clonedSpan := sp
		if sp.Attributes != nil {
			attrs := make(map[string]any)
			for k, v := range sp.Attributes {
				attrs[k] = v
			}
			clonedSpan.Attributes = attrs
		}
		cloned.Trace[i] = clonedSpan
	}
	return cloned
}

// 1. RemoveRequiredToolMutator
type RemoveRequiredToolMutator struct{}

func (m *RemoveRequiredToolMutator) Name() string { return "remove_required_tool" }
func (m *RemoveRequiredToolMutator) Class() ports.MutationClass {
	return ports.MutClassRemoveRequiredTool
}
func (m *RemoveRequiredToolMutator) Mutate(ctx context.Context, run api.AgentRun) (api.MutationOutcome, error) {
	cloned := cloneRun(run)
	toolIdx := -1
	for i, sp := range cloned.Trace {
		if sp.Type == api.SpanTypeTool {
			toolIdx = i
			break
		}
	}
	if toolIdx == -1 {
		return api.MutationOutcome{
			Status:      api.MutationSkipped,
			Class:       string(m.Class()),
			OriginalRun: run,
			SkipReason:  "no tool spans found in trace",
		}, nil
	}

	removedName := cloned.Trace[toolIdx].Name
	cloned.Trace = append(cloned.Trace[:toolIdx], cloned.Trace[toolIdx+1:]...)

	return api.MutationOutcome{
		Status:      api.MutationApplied,
		Class:       string(m.Class()),
		OriginalRun: run,
		MutatedRun:  cloned,
		Description: fmt.Sprintf("removed required tool call %q", removedName),
	}, nil
}

// 2. WrongToolMutator
type WrongToolMutator struct{}

func (m *WrongToolMutator) Name() string               { return "wrong_tool" }
func (m *WrongToolMutator) Class() ports.MutationClass { return ports.MutClassWrongTool }
func (m *WrongToolMutator) Mutate(ctx context.Context, run api.AgentRun) (api.MutationOutcome, error) {
	cloned := cloneRun(run)
	toolIdx := -1
	for i, sp := range cloned.Trace {
		if sp.Type == api.SpanTypeTool {
			toolIdx = i
			break
		}
	}
	if toolIdx == -1 {
		return api.MutationOutcome{
			Status:      api.MutationSkipped,
			Class:       string(m.Class()),
			OriginalRun: run,
			SkipReason:  "no tool spans found in trace",
		}, nil
	}

	origName := cloned.Trace[toolIdx].Name
	cloned.Trace[toolIdx].Name = "wrong_" + origName

	return api.MutationOutcome{
		Status:      api.MutationApplied,
		Class:       string(m.Class()),
		OriginalRun: run,
		MutatedRun:  cloned,
		Description: fmt.Sprintf("replaced tool %q with %q", origName, cloned.Trace[toolIdx].Name),
	}, nil
}

// 3. CorruptArgumentMutator
type CorruptArgumentMutator struct{}

func (m *CorruptArgumentMutator) Name() string               { return "corrupt_argument" }
func (m *CorruptArgumentMutator) Class() ports.MutationClass { return ports.MutClassCorruptArgument }
func (m *CorruptArgumentMutator) Mutate(ctx context.Context, run api.AgentRun) (api.MutationOutcome, error) {
	cloned := cloneRun(run)
	for i, sp := range cloned.Trace {
		if sp.Type == api.SpanTypeTool && sp.Attributes != nil {
			if input, ok := sp.Attributes["input"].(map[string]any); ok && len(input) > 0 {
				corrupted := make(map[string]any)
				for k := range input {
					corrupted[k] = "__CORRUPTED_VALUE__"
				}
				cloned.Trace[i].Attributes["input"] = corrupted
				return api.MutationOutcome{
					Status:      api.MutationApplied,
					Class:       string(m.Class()),
					OriginalRun: run,
					MutatedRun:  cloned,
					Description: fmt.Sprintf("corrupted input arguments of tool %q", sp.Name),
				}, nil
			}
		}
	}
	return api.MutationOutcome{
		Status:      api.MutationSkipped,
		Class:       string(m.Class()),
		OriginalRun: run,
		SkipReason:  "no tool spans with input arguments found",
	}, nil
}

// 4. DuplicateCallMutator
type DuplicateCallMutator struct{}

func (m *DuplicateCallMutator) Name() string               { return "duplicate_call" }
func (m *DuplicateCallMutator) Class() ports.MutationClass { return ports.MutClassDuplicateCall }
func (m *DuplicateCallMutator) Mutate(ctx context.Context, run api.AgentRun) (api.MutationOutcome, error) {
	cloned := cloneRun(run)
	for i, sp := range cloned.Trace {
		if sp.Type == api.SpanTypeTool {
			dup := sp
			dup.SpanID = sp.SpanID + "_dup"
			cloned.Trace = append(cloned.Trace[:i+1], append([]api.Span{dup}, cloned.Trace[i+1:]...)...)
			return api.MutationOutcome{
				Status:      api.MutationApplied,
				Class:       string(m.Class()),
				OriginalRun: run,
				MutatedRun:  cloned,
				Description: fmt.Sprintf("duplicated tool span %q", sp.Name),
			}, nil
		}
	}
	return api.MutationOutcome{
		Status:      api.MutationSkipped,
		Class:       string(m.Class()),
		OriginalRun: run,
		SkipReason:  "no tool spans to duplicate",
	}, nil
}

// 5. InfiniteLoopMutator
type InfiniteLoopMutator struct{}

func (m *InfiniteLoopMutator) Name() string               { return "infinite_loop" }
func (m *InfiniteLoopMutator) Class() ports.MutationClass { return ports.MutClassInfiniteLoop }
func (m *InfiniteLoopMutator) Mutate(ctx context.Context, run api.AgentRun) (api.MutationOutcome, error) {
	cloned := cloneRun(run)
	toolSpan := api.Span{
		SpanID:    "loop_span",
		Name:      "poll_status",
		Type:      api.SpanTypeTool,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(time.Millisecond),
		Status:    api.SpanStatus{Code: "ok"},
	}

	// Inject 30 identical steps
	for i := 0; i < 30; i++ {
		s := toolSpan
		s.SpanID = fmt.Sprintf("loop_%d", i)
		cloned.Trace = append(cloned.Trace, s)
	}

	return api.MutationOutcome{
		Status:      api.MutationApplied,
		Class:       string(m.Class()),
		OriginalRun: run,
		MutatedRun:  cloned,
		Description: "injected infinite loop of 30 tool calls",
	}, nil
}

// 6. SkipRecoveryMutator
type SkipRecoveryMutator struct{}

func (m *SkipRecoveryMutator) Name() string               { return "skip_recovery" }
func (m *SkipRecoveryMutator) Class() ports.MutationClass { return ports.MutClassSkipRecovery }
func (m *SkipRecoveryMutator) Mutate(ctx context.Context, run api.AgentRun) (api.MutationOutcome, error) {
	cloned := cloneRun(run)
	errIdx := -1
	for i, sp := range cloned.Trace {
		if sp.Type == api.SpanTypeError || sp.Status.Code == "error" {
			errIdx = i
			break
		}
	}

	if errIdx == -1 || errIdx == len(cloned.Trace)-1 {
		return api.MutationOutcome{
			Status:      api.MutationSkipped,
			Class:       string(m.Class()),
			OriginalRun: run,
			SkipReason:  "no recoverable error spans followed by recovery steps",
		}, nil
	}

	// Truncate everything after the error span and mark outcome failed
	cloned.Trace = cloned.Trace[:errIdx+1]
	cloned.Outcome.Status = "failed"
	cloned.Outcome.Error = "unhandled error after step truncation"

	return api.MutationOutcome{
		Status:      api.MutationApplied,
		Class:       string(m.Class()),
		OriginalRun: run,
		MutatedRun:  cloned,
		Description: "truncated recovery steps following an error span",
	}, nil
}

// 7. ExcessiveToolCallsMutator
type ExcessiveToolCallsMutator struct{}

func (m *ExcessiveToolCallsMutator) Name() string { return "excessive_tool_calls" }
func (m *ExcessiveToolCallsMutator) Class() ports.MutationClass {
	return ports.MutClassExcessiveToolCalls
}
func (m *ExcessiveToolCallsMutator) Mutate(ctx context.Context, run api.AgentRun) (api.MutationOutcome, error) {
	cloned := cloneRun(run)
	for i := 0; i < 25; i++ {
		cloned.Trace = append(cloned.Trace, api.Span{
			SpanID:    fmt.Sprintf("spam_%d", i),
			Name:      "extra_unneeded_tool",
			Type:      api.SpanTypeTool,
			StartTime: time.Now(),
			EndTime:   time.Now().Add(time.Millisecond),
			Status:    api.SpanStatus{Code: "ok"},
		})
	}
	return api.MutationOutcome{
		Status:      api.MutationApplied,
		Class:       string(m.Class()),
		OriginalRun: run,
		MutatedRun:  cloned,
		Description: "injected 25 excessive tool calls",
	}, nil
}

// 8. IntroduceForbiddenToolMutator
type IntroduceForbiddenToolMutator struct{}

func (m *IntroduceForbiddenToolMutator) Name() string { return "introduce_forbidden_tool" }
func (m *IntroduceForbiddenToolMutator) Class() ports.MutationClass {
	return ports.MutClassIntroduceForbidden
}
func (m *IntroduceForbiddenToolMutator) Mutate(ctx context.Context, run api.AgentRun) (api.MutationOutcome, error) {
	cloned := cloneRun(run)
	cloned.Trace = append(cloned.Trace, api.Span{
		SpanID:    "forbidden_01",
		Name:      "forbidden_admin_access",
		Type:      api.SpanTypeTool,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(time.Millisecond),
		Status:    api.SpanStatus{Code: "ok"},
		Attributes: map[string]any{
			"input": map[string]any{"cmd": "drop table users"},
		},
	})
	return api.MutationOutcome{
		Status:      api.MutationApplied,
		Class:       string(m.Class()),
		OriginalRun: run,
		MutatedRun:  cloned,
		Description: "injected forbidden_admin_access tool call",
	}, nil
}

// 9. ChangeFinalOutputMutator
type ChangeFinalOutputMutator struct{}

func (m *ChangeFinalOutputMutator) Name() string { return "change_final_output" }
func (m *ChangeFinalOutputMutator) Class() ports.MutationClass {
	return ports.MutClassChangeFinalOutput
}
func (m *ChangeFinalOutputMutator) Mutate(ctx context.Context, run api.AgentRun) (api.MutationOutcome, error) {
	cloned := cloneRun(run)
	cloned.Outcome.Output = "The operation completely failed."
	return api.MutationOutcome{
		Status:      api.MutationApplied,
		Class:       string(m.Class()),
		OriginalRun: run,
		MutatedRun:  cloned,
		Description: "changed final output string",
	}, nil
}

// 10. SwapAgentOrderMutator — breaks multi-agent coordination order.
type SwapAgentOrderMutator struct{}

func (m *SwapAgentOrderMutator) Name() string { return "swap_agent_order" }
func (m *SwapAgentOrderMutator) Class() ports.MutationClass {
	return ports.MutClassWrongTool // reuse class bucket; name is authoritative in breakdown
}
func (m *SwapAgentOrderMutator) Mutate(ctx context.Context, run api.AgentRun) (api.MutationOutcome, error) {
	cloned := cloneRun(run)
	idxs := make([]int, 0)
	for i, sp := range cloned.Trace {
		if sp.Type == api.SpanTypeAgent {
			idxs = append(idxs, i)
		}
	}
	if len(idxs) < 2 {
		return api.MutationOutcome{
			Status: api.MutationSkipped, Class: string(m.Class()), OriginalRun: run,
			SkipReason: "need at least two agent spans",
		}, nil
	}
	i, j := idxs[0], idxs[1]
	cloned.Trace[i], cloned.Trace[j] = cloned.Trace[j], cloned.Trace[i]
	return api.MutationOutcome{
		Status: api.MutationApplied, Class: string(m.Class()), OriginalRun: run, MutatedRun: cloned,
		Description: "swapped first two agent spans",
	}, nil
}

// AllBuiltinMutators returns the complete list of mutator instances.
func AllBuiltinMutators() []ports.Mutator {
	return []ports.Mutator{
		&RemoveRequiredToolMutator{},
		&WrongToolMutator{},
		&CorruptArgumentMutator{},
		&DuplicateCallMutator{},
		&InfiniteLoopMutator{},
		&SkipRecoveryMutator{},
		&ExcessiveToolCallsMutator{},
		&IntroduceForbiddenToolMutator{},
		&ChangeFinalOutputMutator{},
		&SwapAgentOrderMutator{},
	}
}
