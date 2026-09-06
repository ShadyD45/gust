package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"gust/internal/adapters/evaluators"
	"gust/internal/adapters/mutators"
	"gust/internal/adapters/testrunner"
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

func loadScenario(path string) (api.TestScenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return api.TestScenario{}, err
	}
	var sc api.TestScenario
	if err := yaml.Unmarshal(data, &sc); err != nil {
		if err2 := json.Unmarshal(data, &sc); err2 != nil {
			return api.TestScenario{}, fmt.Errorf("parse scenario: %v / %v", err, err2)
		}
	}
	return sc, nil
}

func builtinEvaluators() []ports.Evaluator {
	return evaluators.AllBuiltinEvaluators()
}

func builtinMutators() []ports.Mutator {
	return mutators.AllBuiltinMutators()
}

func resolveRunner(name, endpoint, model string, passProb float64) (ports.TestRunner, error) {
	switch name {
	case "synthetic", "":
		return testrunner.NewSyntheticRunner(passProb, 42), nil
	case "ollama":
		return testrunner.NewOllamaRunner(endpoint, model), nil
	default:
		if tr, ok := registry.DefaultRegistry.GetTestRunner(name); ok {
			return tr, nil
		}
		return nil, fmt.Errorf("unknown runner %q", name)
	}
}
