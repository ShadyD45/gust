package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"gust/internal/adapters/evaluators"
	"gust/internal/adapters/mutators"
	"gust/internal/ports"
	"gust/internal/registry"
	"gust/pkg/api"
)

// Execute runs the root command.
func Execute() int {
	root := NewRoot()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return api.ExitConfigError
	}
	return 0
}

// NewRoot builds the gust command tree.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "gust",
		Short:         "Test infrastructure for autonomous agents",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.AddCommand(
		newAnalyzeCmd(),
		newReplayCmd(),
		newTestCmd(),
		newMutateCmd(),
		newCompareCmd(),
		newScenarioCmd(),
		newIngestCmd(),
		newJudgeCmd(),
		newAEECmd(),
	)
	return root
}

func loadJSON[T any](path string) (T, error) {
	var zero T
	data, err := os.ReadFile(path)
	if err != nil {
		return zero, err
	}
	if err := json.Unmarshal(data, &zero); err != nil {
		return zero, fmt.Errorf("parse %s: %w", path, err)
	}
	return zero, nil
}

func loadPolicy(path string) (api.Policy, error) {
	if path == "" {
		return api.Policy{
			Version: "1.0",
			Name:    "default",
			Reliability: api.PolicyReliability{
				DefaultMinimumPassRate: 0.95,
				MinSamplesForVerdict:   5,
				OnFlaky:                "warn",
			},
		}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return api.Policy{}, err
	}
	var p api.Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		// try JSON
		if err2 := json.Unmarshal(data, &p); err2 != nil {
			return api.Policy{}, fmt.Errorf("parse policy: %v / %v", err, err2)
		}
	}
	if err := p.Validate(); err != nil {
		return api.Policy{}, err
	}
	return p, nil
}

// activeEvaluators returns the built-in suite merged with registry entries.
// Registry entries override builtins of the same name (so a Tier-2 llm_judge
// plugin using an official provider SDK replaces the Go generic adapter).
func activeEvaluators() []ports.Evaluator {
	byName := make(map[string]ports.Evaluator)
	var order []string
	for _, e := range evaluators.AllBuiltinEvaluators() {
		byName[e.Name()] = e
		order = append(order, e.Name())
	}
	for _, e := range registry.DefaultRegistry.ListEvaluators() {
		if _, exists := byName[e.Name()]; !exists {
			order = append(order, e.Name())
		}
		byName[e.Name()] = e
	}
	list := make([]ports.Evaluator, 0, len(order))
	for _, name := range order {
		list = append(list, byName[name])
	}
	return list
}

// activeMutators returns the built-in mutation classes plus any registered externally.
func activeMutators() []ports.Mutator {
	list := mutators.AllBuiltinMutators()
	seen := make(map[string]struct{}, len(list))
	for _, m := range list {
		seen[m.Name()] = struct{}{}
	}
	for _, m := range registry.DefaultRegistry.ListMutators() {
		if _, dup := seen[m.Name()]; dup {
			continue
		}
		list = append(list, m)
	}
	return list
}
