package cli

import (
	"os"
	"path/filepath"

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

	cmd := &cobra.Command{
		Use:   "from-run <run.json>",
		Short: "Extract a TestScenario from an AgentRun (assertions left empty)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			run, err := loadJSON[api.AgentRun](args[0])
			if err != nil {
				return err
			}
			sc, yamlBytes, err := scenario.NewExtractor().ExtractYAML(run)
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
	cmd.Flags().StringVar(&output, "output", "", "write YAML to file")
	return cmd
}
