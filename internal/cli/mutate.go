package cli

import (
	"context"
	"encoding/json"
	"os"

	"github.com/spf13/cobra"

	"gust/internal/core/mutate"
	"gust/pkg/api"
)

func newMutateCmd() *cobra.Command {
	var asJSON bool
	var n int

	cmd := &cobra.Command{
		Use:   "mutate <run.json>",
		Short: "Run mutation testing against a golden AgentRun",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			run, err := loadJSON[api.AgentRun](args[0])
			if err != nil {
				return err
			}
			assertions, err := loadAssertions(run, "")
			if err != nil {
				return err
			}

			_ = n // reserved for future multi-apply; MVP applies each mutator once per case
			runner := mutate.NewRunner(builtinMutators(), builtinEvaluators())
			report, err := runner.RunBenchmark(context.Background(), []mutate.GoldenCase{
				{Run: run, Assertions: assertions},
			})
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			PrintMutateTerminal(report)
			if report.DetectionRate < 0.90 || report.FalsePositiveRate > 0.05 {
				os.Exit(api.ExitFailure)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&n, "n", 100, "target mutant count hint (informational)")
	cmd.Flags().String("classes", "all", "mutation classes (all)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "machine-readable JSON output")
	return cmd
}
