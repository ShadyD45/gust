package cli

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"

	"gust/internal/core/policy"
	"gust/pkg/api"
)

func newCompareCmd() *cobra.Command {
	var policyPath string
	var asJSON bool
	var bootstrapN int
	var seed int64

	cmd := &cobra.Command{
		Use:   "compare <baseline.json> <candidate.json>",
		Short: "Baseline vs candidate regression comparison",
		Long: `Compare baseline vs candidate pass rates with a two-proportion z-test gate.

Also reports Cohen's d effect size and a seedable bootstrap CI for the
pass-rate difference (stats v2 diagnostics). Mode 3 single-scenario verdicts
still use closed-form Wilson intervals.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			base, err := loadJSON[policy.ExperimentStats](args[0])
			if err != nil {
				return err
			}
			cand, err := loadJSON[policy.ExperimentStats](args[1])
			if err != nil {
				return err
			}
			pol, err := loadPolicy(policyPath)
			if err != nil {
				return err
			}

			reg := policy.CompareRegressionOpts(base, cand, pol.Regression, policy.CompareOptions{
				BootstrapReplicates: bootstrapN,
				BootstrapSeed:       seed,
			})
			verdict := policy.NewEngine().EvaluateWithRegression(pol, nil, &reg)

			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"regression": reg, "policy": verdict})
			}
			PrintRegressionTerminal(&reg)
			if verdict.ExitCode != api.ExitSuccess {
				os.Exit(verdict.ExitCode)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&policyPath, "policy", "", "policy YAML/JSON")
	cmd.Flags().BoolVar(&asJSON, "json", false, "machine-readable JSON output")
	cmd.Flags().IntVar(&bootstrapN, "bootstrap", 1000, "bootstrap replicates for diff CI")
	cmd.Flags().Int64Var(&seed, "seed", 42, "RNG seed for bootstrap")
	return cmd
}
