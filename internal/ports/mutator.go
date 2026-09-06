package ports

import (
	"context"
	"gust/pkg/api"
)

type MutationClass string

const (
	MutClassRemoveRequiredTool   MutationClass = "remove_required_tool"
	MutClassWrongTool            MutationClass = "wrong_tool"
	MutClassCorruptArgument      MutationClass = "corrupt_argument"
	MutClassDuplicateCall        MutationClass = "duplicate_call"
	MutClassInfiniteLoop         MutationClass = "infinite_loop"
	MutClassSkipRecovery         MutationClass = "skip_recovery"
	MutClassExcessiveToolCalls   MutationClass = "excessive_tool_calls"
	MutClassIntroduceForbidden   MutationClass = "introduce_forbidden_tool"
	MutClassChangeFinalOutput    MutationClass = "change_final_output"
)

// Mutator defines the interface for injecting mutations into an AgentRun.
type Mutator interface {
	Name() string
	Class() MutationClass
	Mutate(ctx context.Context, run api.AgentRun) (api.MutationOutcome, error)
}
