package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"gust/internal/core/analyze"
	"gust/pkg/api"
)

func newAnalyzeCmd() *cobra.Command {
	var policyPath string
	var asJSON bool
	var assertionsPath string
	var judgePlugin string
	var plugins []string

	cmd := &cobra.Command{
		Use:   "analyze <run.json>",
		Short: "Mode 1: evaluate assertions against a captured AgentRun",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cleanup, err := loadPluginsForCommand(plugins, judgePlugin)
			if err != nil {
				return err
			}
			defer cleanup()

			run, err := loadJSON[api.AgentRun](args[0])
			if err != nil {
				return err
			}
			pol, err := loadPolicy(policyPath)
			if err != nil {
				return err
			}

			assertions, err := loadAssertions(run, assertionsPath)
			if err != nil {
				return err
			}

			engine := analyze.NewEngine(activeEvaluators())
			report, err := engine.AnalyzeRun(context.Background(), run, assertions, evalContextFromPolicy(pol, run.RunID))
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				if err := enc.Encode(report); err != nil {
					return err
				}
			} else {
				PrintAnalyzeTerminal(report)
			}

			if !report.Passed {
				os.Exit(api.ExitFailure)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&policyPath, "policy", "", "policy YAML/JSON")
	cmd.Flags().StringVar(&assertionsPath, "assertions", "", "optional assertions JSON file")
	cmd.Flags().BoolVar(&asJSON, "json", false, "machine-readable JSON output")
	cmd.Flags().StringVar(&judgePlugin, "judge-plugin", "", "Tier-2 LLM judge plugin (deprecated alias of --plugin with role judge)")
	cmd.Flags().StringArrayVar(&plugins, "plugin", nil, "Tier-2 evaluator plugin (`python path.py` or `alias=python path.py`); repeatable")
	return cmd
}

func loadAssertions(run api.AgentRun, path string) ([]api.Assertion, error) {
	if path != "" {
		return loadJSON[[]api.Assertion](path)
	}
	if run.Metadata != nil {
		if raw, ok := run.Metadata["assertions"]; ok {
			b, err := json.Marshal(raw)
			if err != nil {
				return nil, err
			}
			var assertions []api.Assertion
			if err := json.Unmarshal(b, &assertions); err != nil {
				return nil, fmt.Errorf("metadata assertions: %w", err)
			}
			return assertions, nil
		}
	}
	return []api.Assertion{{ID: "default_success", Type: api.AssertTaskSuccess}}, nil
}
