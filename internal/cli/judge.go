package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"gust/internal/adapters/judge"
	corejudge "gust/internal/core/judge"
	"gust/pkg/api"
)

func newJudgeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "judge",
		Short: "Optional LLM judge utilities (calibration)",
	}
	cmd.AddCommand(newJudgeCalibrateCmd())
	return cmd
}

func newJudgeCalibrateCmd() *cobra.Command {
	var datasetPath string
	var providerName string
	var endpoint string
	var model string
	var apiKey string
	var minRho float64
	var asJSON bool
	var outPath string

	cmd := &cobra.Command{
		Use:   "calibrate",
		Short: "Compute Spearman ρ of a judge against a labeled calibration dataset",
		Long: `Runs a JudgeProvider over a versioned calibration set and reports Spearman ρ.

Enablement gate: ρ ≥ 0.7 on ≥ 50 cases. Until that gate passes, llm_judge remains
experimental and soft-only (policy allow_llm_judge still required to invoke it).

Example:
  gust judge calibrate --dataset testdata/judge/calibration/v1.json --provider mock
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if datasetPath == "" {
				datasetPath = "testdata/judge/calibration/v1.json"
			}
			ds, err := corejudge.LoadDataset(datasetPath)
			if err != nil {
				return err
			}
			if len(ds.Cases) < corejudge.DefaultMinCalibrationCases {
				fmt.Fprintf(os.Stderr, "warning: dataset has %d cases; gate requires ≥%d\n",
					len(ds.Cases), corejudge.DefaultMinCalibrationCases)
			}
			prov, err := judge.ResolveProvider(providerName, endpoint, model, apiKey)
			if err != nil {
				return err
			}
			report, err := corejudge.Calibrate(context.Background(), ds, prov, minRho)
			if err != nil {
				return err
			}
			if model != "" {
				report.Model = model
			}

			if outPath != "" {
				b, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return err
				}
				if err := os.WriteFile(outPath, b, 0o644); err != nil {
					return err
				}
			}

			md := formatJudgeCalibrationMarkdown(report)
			appendGitHubSummary(md)

			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				if err := enc.Encode(report); err != nil {
					return err
				}
			} else {
				fmt.Printf("Judge calibration\n")
				fmt.Printf("  provider:     %s\n", report.Provider)
				fmt.Printf("  dataset hash: %s\n", report.DatasetHash)
				fmt.Printf("  n:            %d\n", report.N)
				fmt.Printf("  spearman ρ:   %.4f (min %.2f)\n", report.SpearmanRho, report.MinSpearman)
				if report.Passed {
					fmt.Printf("  gate:         PASS — may set policy llm_judge.calibrated: true\n")
				} else {
					fmt.Printf("  gate:         FAIL — judge remains experimental / soft-only\n")
				}
				if len(report.Errors) > 0 {
					fmt.Printf("  errors:       %d\n", len(report.Errors))
				}
			}

			if !report.Passed {
				os.Exit(api.ExitFailure)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&datasetPath, "dataset", "testdata/judge/calibration/v1.json", "calibration dataset JSON")
	cmd.Flags().StringVar(&providerName, "provider", "mock", "built-in provider: mock|generic (major clouds use --judge-plugin + official SDKs)")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "generic provider base URL")
	cmd.Flags().StringVar(&model, "model", "", "model id")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for generic provider (or OPENAI_API_KEY / GUST_JUDGE_API_KEY)")
	cmd.Flags().Float64Var(&minRho, "min-rho", corejudge.DefaultMinSpearman, "minimum Spearman ρ to pass the gate")
	cmd.Flags().BoolVar(&asJSON, "json", false, "machine-readable JSON output")
	cmd.Flags().StringVar(&outPath, "out", "", "write full report JSON to path")
	return cmd
}

func formatJudgeCalibrationMarkdown(report *corejudge.CalibrationReport) string {
	gate := "**PASS**"
	if !report.Passed {
		gate = "**FAIL** (experimental / soft-only)"
	}
	return fmt.Sprintf(`## Judge calibration

| Field | Value |
| --- | --- |
| Provider | %s |
| Dataset hash | %s |
| N | %d |
| Spearman ρ | **%.4f** (min %.2f) |
| Gate | %s |

`, report.Provider, report.DatasetHash, report.N, report.SpearmanRho, report.MinSpearman, gate)
}
