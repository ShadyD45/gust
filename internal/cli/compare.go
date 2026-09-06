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

	cmd := &cobra.Command{
		Use:   "compare <baseline.json> <candidate.json>",
		Short: "Baseline vs candidate regression comparison",
		Args:  cobra.ExactArgs(2),
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

			reg := policy.CompareRegression(base, cand, pol.Regression)
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
	return cmd
}
