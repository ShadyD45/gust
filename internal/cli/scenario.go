package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"gust/internal/core/redact"
	"gust/internal/core/scenario"
	"gust/pkg/api"
)

func newScenarioCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scenario",
		Short: "Scenario extraction and management",
	}
	cmd.AddCommand(newScenarioFromRunCmd())
	cmd.AddCommand(newScenarioProposeCmd())
	return cmd
}

func newScenarioFromRunCmd() *cobra.Command {
	var output string
	var layout string

	cmd := &cobra.Command{
		Use:   "from-run <run.json>",
		Short: "Extract a TestScenario from an AgentRun (assertions left empty)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			run, err := loadJSON[api.AgentRun](args[0])
			if err != nil {
				return err
			}
			ext := scenario.NewExtractor()
			if strings.EqualFold(layout, "dir") {
				if output == "" {
					return fmt.Errorf("--output is required with --layout dir")
				}
				sc, err := ext.ExtractFromRun(run)
				if err != nil {
					return err
				}
				return writeScenarioDir(output, sc)
			}
			sc, yamlBytes, err := ext.ExtractYAML(run)
			if err != nil {
				return err
			}
			_ = sc
			if output == "" {
				_, err := os.Stdout.Write(yamlBytes)
				return err
			}
			if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil && filepath.Dir(output) != "." {
				return err
			}
			return os.WriteFile(output, yamlBytes, 0644)
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "write YAML to file, or a directory when --layout dir")
	cmd.Flags().StringVar(&layout, "layout", "file", "file (single YAML) or dir (scenario.yaml + fixtures/)")
	return cmd
}

func writeScenarioDir(dir string, sc *api.TestScenario) error {
	if err := os.MkdirAll(filepath.Join(dir, "fixtures"), 0755); err != nil {
		return err
	}
	for _, fx := range sc.Environment.Fixtures {
		encoded, err := json.MarshalIndent(fx, "", "  ")
		if err != nil {
			return err
		}
		name := fx.FixtureID + ".json"
		if err := os.WriteFile(filepath.Join(dir, "fixtures", name), append(encoded, '\n'), 0644); err != nil {
			return err
		}
	}
	sc.Environment.Fixtures = []api.Fixture{}
	sc.Environment.FixturesDir = "fixtures"
	yamlBytes, err := scenario.RenderYAML(sc)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "scenario.yaml"), yamlBytes, 0644)
}

func newScenarioProposeCmd() *cobra.Command {
	var outputDir string
	var noRedact bool
	var optOutNote string
	var customRules []string
	var limit int
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "propose <run.json|runs-dir>",
		Short: "Propose reviewable TestScenario drafts from production traces",
		Long: `Sample one AgentRun file or a directory of runs into candidate scenarios.

Redaction is ON by default (emails, phones, tokens, custom rules). Opting out
requires --redact-opt-out-note for audit. Assertions are never auto-filled from
behavior; reviewed_by stays empty until a human reviews the draft.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if noRedact && strings.TrimSpace(optOutNote) == "" {
				return fmt.Errorf("--no-redact requires --redact-opt-out-note")
			}
			rules, err := parseProposeRules(customRules)
			if err != nil {
				return err
			}
			runs, err := loadProposeRuns(args[0], limit)
			if err != nil {
				return err
			}
			if len(runs) == 0 {
				return fmt.Errorf("no valid AgentRun documents found in %s", args[0])
			}
			ext := scenario.NewExtractor()
			opts := scenario.ProposeOptions{
				Redact:           !noRedact,
				RedactOptOutNote: optOutNote,
				Rules:            rules,
			}
			var proposals []*scenario.Proposal
			for _, run := range runs {
				p, err := ext.ProposeFromRun(run, opts)
				if err != nil {
					return err
				}
				proposals = append(proposals, p)
			}
			if outputDir != "" {
				if err := os.MkdirAll(outputDir, 0755); err != nil {
					return err
				}
				for i, p := range proposals {
					dir := filepath.Join(outputDir, p.Scenario.ID)
					if err := writeScenarioDir(dir, p.Scenario); err != nil {
						return err
					}
					meta, _ := json.MarshalIndent(map[string]any{
						"redaction":       p.Redaction,
						"proposed_at":     p.ProposedAt,
						"assertions_note": p.AssertionsNote,
						"index":           i,
					}, "", "  ")
					_ = os.WriteFile(filepath.Join(dir, "proposal.json"), append(meta, '\n'), 0644)
				}
				fmt.Fprintf(os.Stderr, "wrote %d proposal(s) under %s (reviewed_by empty; assertions empty)\n", len(proposals), outputDir)
				return nil
			}
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(proposals)
			}
			for _, p := range proposals {
				yamlBytes, err := scenario.RenderYAML(p.Scenario)
				if err != nil {
					return err
				}
				_, _ = os.Stdout.Write(yamlBytes)
				fmt.Fprintln(os.Stdout, "---")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "write one scenario dir per proposal")
	cmd.Flags().BoolVar(&noRedact, "no-redact", false, "disable default PII redaction (requires --redact-opt-out-note)")
	cmd.Flags().StringVar(&optOutNote, "redact-opt-out-note", "", "audit reason when redaction is disabled")
	cmd.Flags().StringArrayVar(&customRules, "rule", nil, "custom redaction rule name:pattern[:replacement]")
	cmd.Flags().IntVar(&limit, "limit", 0, "max runs to sample from a directory (0 = all)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit proposals as JSON")
	return cmd
}

func parseProposeRules(specs []string) ([]redact.Rule, error) {
	if len(specs) == 0 {
		return redact.DefaultRules(), nil
	}
	custom, err := redact.ParseCustomRules(specs)
	if err != nil {
		return nil, err
	}
	return append(redact.DefaultRules(), custom...), nil
}

func loadProposeRuns(path string, limit int) ([]api.AgentRun, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return scenario.SampleRuns(path, limit)
	}
	run, err := loadJSON[api.AgentRun](path)
	if err != nil {
		return nil, err
	}
	return []api.AgentRun{run}, nil
}
