package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"gust/internal/core/stats"
)

func newRecommendSamplesCmd() *cobra.Command {
	var minPass float64
	var confidence float64
	var expected float64
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "recommend-samples",
		Short: "Recommend Mode 3 sample counts for a reliability floor",
		Long: `Compute the smallest N such that a Wilson PASS is attainable at the given
minimum pass rate and confidence. Avoids unattainable policies (e.g. N=20
never PASS at P_min=0.95). Wilson remains the Mode 3 verdict engine.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			perfect, expectedN, err := stats.RecommendSamples(minPass, confidence, expected)
			if err != nil {
				return err
			}
			out := map[string]any{
				"min_pass_rate":       minPass,
				"confidence":          confidence,
				"expected_pass_rate":  expected,
				"n_perfect_run":       perfect,
				"n_at_expected_rate":  expectedN,
				"note":                "Wilson remains the default Mode 3 verdict path",
			}
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			fmt.Printf("recommend-samples\n")
			fmt.Printf("  min_pass_rate=%.2f  confidence=%.2f  expected_rate=%.2f\n", minPass, confidence, expected)
			fmt.Printf("  N for perfect run (n/n):     %d\n", perfect)
			fmt.Printf("  N at expected pass rate:      %d\n", expectedN)
			return nil
		},
	}
	cmd.Flags().Float64Var(&minPass, "min-pass-rate", 0.95, "policy minimum pass rate (P_min)")
	cmd.Flags().Float64Var(&confidence, "confidence", 0.95, "Wilson confidence level")
	cmd.Flags().Float64Var(&expected, "expected-rate", 1.0, "assumed true pass rate when sizing N")
	cmd.Flags().BoolVar(&asJSON, "json", false, "machine-readable JSON output")
	return cmd
}
