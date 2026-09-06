package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"gust/internal/core/aee"
	"gust/pkg/api"
)

func newAEECmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aee",
		Short: "Agent Evaluation Effectiveness self-benchmarks",
	}
	cmd.AddCommand(newAEEReportCmd())
	return cmd
}

func newAEEReportCmd() *cobra.Command {
	var asJSON bool
	var asMarkdown bool
	var outPath string
	var docPath string
	var siteDocPath string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Run offline AEE self-score (mutation, throughput, H7, reproducibility)",
		Long: `Measures gust's own evaluation suite — not an agent under test.

Inputs (Replay / offline only):
  - mutation detection rate & false positive rate
  - deterministic evaluator throughput
  - analyze reproducibility (JCS hash stability)
  - H7 Wilson verdict vectors (PASS / FLAKY / FAIL)

Self-benchmark only; see docs/benchmarks/ and docs/usage/aee-methodology.md.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rep, err := aee.Run(context.Background())
			if err != nil {
				return err
			}
			if outPath != "" {
				b, err := json.MarshalIndent(rep, "", "  ")
				if err != nil {
					return err
				}
				if err := os.WriteFile(outPath, b, 0o644); err != nil {
					return err
				}
			}
			if docPath != "" {
				if err := os.WriteFile(docPath, []byte(aee.FormatDocMarkdown(rep)), 0o644); err != nil {
					return err
				}
			}
			if siteDocPath != "" {
				if err := os.WriteFile(siteDocPath, []byte(aee.FormatSiteResultsMarkdown(rep)), 0o644); err != nil {
					return err
				}
			}

			md := aee.FormatMarkdown(rep)
			appendGitHubSummary(md)

			switch {
			case asJSON:
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				if err := enc.Encode(rep); err != nil {
					return err
				}
			case asMarkdown:
				fmt.Print(md)
			default:
				fmt.Printf("gust AEE self-report\n")
				fmt.Printf("  detection_rate:      %.2f%%  (min %.0f%%)\n", rep.DetectionRate*100, aee.MinDetectionRate*100)
				fmt.Printf("  false_positive_rate: %.2f%%  (max %.0f%%)\n", rep.FalsePositiveRate*100, aee.MaxFalsePositive*100)
				fmt.Printf("  eval_throughput:     %s cases/sec  (min %.0f)\n", aee.FormatThroughput(rep.EvalThroughputCPS), aee.MinEvalThroughput)
				fmt.Printf("  reproducibility:     %.0f%% identical hashes over %d trials\n", rep.Reproducibility*100, aee.ReproTrials)
				fmt.Printf("  H7 pass:             %s\n", rep.H7.PassCase)
				fmt.Printf("  H7 flaky:            %s\n", rep.H7.FlakyCase)
				fmt.Printf("  H7 fail:             %s\n", rep.H7.FailCase)
				fmt.Printf("  aee_score:           %.3f\n", rep.Score)
				if rep.Passed {
					fmt.Printf("  gate:                PASS\n")
				} else {
					fmt.Printf("  gate:                FAIL\n")
					for _, n := range rep.Notes {
						fmt.Printf("    - %s\n", n)
					}
				}
			}
			if !rep.Passed {
				os.Exit(api.ExitFailure)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "machine-readable JSON")
	cmd.Flags().BoolVar(&asMarkdown, "markdown", false, "print GitHub-flavored markdown")
	cmd.Flags().StringVar(&outPath, "out", "", "write report JSON to path")
	cmd.Flags().StringVar(&docPath, "doc", "", "write benchmarks/RESULTS.md-style markdown to path")
	cmd.Flags().StringVar(&siteDocPath, "site-doc", "", "write docs/benchmarks/results.md (Just the Docs front matter)")
	return cmd
}

func appendGitHubSummary(md string) {
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" || md == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(md)
	if !stringsHasSuffixNewline(md) {
		_, _ = f.WriteString("\n")
	}
}

func stringsHasSuffixNewline(s string) bool {
	return len(s) > 0 && s[len(s)-1] == '\n'
}
