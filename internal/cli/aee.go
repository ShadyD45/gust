package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"gust/internal/core/aee"
	"gust/internal/core/validation"
	"gust/pkg/api"
)

// ExecuteAEE runs the internal gust-aee command tree (self-benchmarks only).
func ExecuteAEE() int {
	root := NewAEERoot()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return api.ExitConfigError
	}
	return 0
}

// NewAEERoot builds the maintainer/CI binary: gust-aee report | validate.
// Not published to end users via package managers — see Phase 19 releases plan.
func NewAEERoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "gust-aee",
		Short: "Internal self-benchmarks for gust (maintainer/CI)",
		Long: `Agent Evaluation Effectiveness and the Gust Validation Suite.

These commands measure gust itself — mutation DR/FPR, Wilson vectors, adversarial
cases. They are for maintainers and CI only.

End users should install and run "gust" (analyze / replay / test / …), not gust-aee.
`,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.AddCommand(newAEEReportCmd())
	root.AddCommand(newAEEValidateCmd())
	return root
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
				fmt.Printf("gust-aee report\n")
				fmt.Printf("  detection_rate:      %.2f%%  (min %.0f%%)\n", rep.DetectionRate*100, aee.MinDetectionRate*100)
				fmt.Printf("  false_positive_rate: %.2f%%  (max %.0f%%)\n", rep.FalsePositiveRate*100, aee.MaxFalsePositive*100)
				fmt.Printf("  eval_throughput:     %s evaluator calls/sec  (min %.0f)\n", aee.FormatThroughput(rep.EvalThroughputCPS), aee.MinEvalThroughput)
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

func newAEEValidateCmd() *cobra.Command {
	var asJSON bool
	var asMarkdown bool
	var outPath string
	var docPath string
	var siteDocPath string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Run Gust Validation Suite (adversarial self-trust cases)",
		Long: `Maintainer/CI only. Proves Gust classifies adversarial scenarios correctly:

  Should this pass / fail / be flaky?
  Should this be an infrastructure error?
  Should this mutation be detected?
  Should this fixture match?
  Should this be considered recovery?

Offline only. Exit 1 if any case mismatches its expected outcome.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rep, err := validation.Run(context.Background())
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
				if err := os.WriteFile(docPath, []byte(validation.FormatDocMarkdown(rep)), 0o644); err != nil {
					return err
				}
			}
			if siteDocPath != "" {
				if err := os.WriteFile(siteDocPath, []byte(validation.FormatSiteResultsMarkdown(rep)), 0o644); err != nil {
					return err
				}
			}

			md := validation.FormatMarkdown(rep)
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
				fmt.Printf("gust-aee validate\n")
				fmt.Printf("  suite_version: %s\n", rep.SuiteVersion)
				fmt.Printf("  cases:         %d/%d passed (%.1f%%)\n", rep.Passed, rep.Total, rep.PassRate*100)
				for _, cat := range []string{"pass", "fail", "flaky", "infra", "mutation", "fixture", "recovery", "mode3"} {
					st, ok := rep.ByCategory[cat]
					if !ok {
						continue
					}
					fmt.Printf("  %-12s %d/%d\n", cat+":", st.Passed, st.Total)
				}
				if rep.GatePassed {
					fmt.Printf("  gate:          PASS\n")
				} else {
					fmt.Printf("  gate:          FAIL\n")
					for _, f := range rep.Failures {
						fmt.Printf("    - %s: %s\n", f.ID, f.Detail)
					}
				}
			}
			if !rep.GatePassed {
				os.Exit(api.ExitFailure)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "machine-readable JSON")
	cmd.Flags().BoolVar(&asMarkdown, "markdown", false, "print GitHub-flavored markdown")
	cmd.Flags().StringVar(&outPath, "out", "", "write report JSON to path")
	cmd.Flags().StringVar(&docPath, "doc", "", "write benchmarks/VALIDATION.md-style markdown")
	cmd.Flags().StringVar(&siteDocPath, "site-doc", "", "write docs/benchmarks/validation.md")
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
