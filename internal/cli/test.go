package cli

import (
	"context"
	"encoding/json"
	"os"

	"github.com/spf13/cobra"

	coretest "gust/internal/core/test"
	"gust/internal/core/policy"
	"gust/pkg/api"
)

func newTestCmd() *cobra.Command {
	var samples int
	var concurrency int
	var runnerName string
	var endpoint string
	var model string
	var policyPath string
	var passProb float64
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "test <scenario.yaml>",
		Short: "Mode 3: probabilistic testing with Wilson reliability",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sc, err := loadScenario(args[0])
			if err != nil {
				return err
			}
			if samples > 0 {
				sc.Reliability.Samples = samples
			}
			if sc.Reliability.Confidence == 0 {
				sc.Reliability.Confidence = 0.95
			}
			if sc.Reliability.MinimumPassRate == 0 {
				sc.Reliability.MinimumPassRate = 0.95
			}
			if len(sc.Assertions) == 0 {
				sc.Assertions = []api.Assertion{{ID: "default_success", Type: api.AssertTaskSuccess}}
			}

			pol, err := loadPolicy(policyPath)
			if err != nil {
				return err
			}

			runner, err := resolveRunner(runnerName, endpoint, model, passProb)
			if err != nil {
				return err
			}

			sampler := coretest.NewSampler(builtinEvaluators())
			result, err := sampler.RunScenario(context.Background(), coretest.SamplingConfig{
				Scenario:    sc,
				Runner:      runner,
				Concurrency: concurrency,
				Endpoint:    endpoint,
				MinSamples:  pol.Reliability.MinSamplesForVerdict,
			})
			if err != nil {
				return err
			}

			eng := policy.NewEngine()
			verdict := eng.Evaluate(pol, []*api.ReliabilityResult{result})

			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(map[string]any{
					"reliability": result,
					"policy":      verdict,
				})
			} else {
				PrintReliabilityTerminal(result)
				WriteGitHubSummary([]*api.ReliabilityResult{result})
			}

			if verdict.ExitCode != api.ExitSuccess {
				os.Exit(verdict.ExitCode)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&samples, "samples", 0, "override scenario sample count")
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "parallel workers")
	cmd.Flags().StringVar(&runnerName, "runner", "synthetic", "test runner: synthetic|ollama")
	cmd.Flags().StringVar(&endpoint, "endpoint", "http://localhost:11434", "runner endpoint")
	cmd.Flags().StringVar(&model, "model", "llama3.1:8b", "ollama model")
	cmd.Flags().Float64Var(&passProb, "pass-probability", 1.0, "synthetic runner pass probability")
	cmd.Flags().StringVar(&policyPath, "policy", "", "policy YAML/JSON")
	cmd.Flags().BoolVar(&asJSON, "json", false, "machine-readable JSON output")
	return cmd
}
