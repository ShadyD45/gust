package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"gust/internal/core/scenario"
	"gust/pkg/api"
)

func newScenarioCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scenario",
		Short: "Scenario extraction and management",
	}
	cmd.AddCommand(newScenarioFromRunCmd())
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
