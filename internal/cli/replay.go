package cli

import (
	"context"
	"encoding/json"
	"os"

	"github.com/spf13/cobra"

	"gust/internal/core/replay"
	"gust/pkg/api"
)

func newReplayCmd() *cobra.Command {
	var fixturesDir string
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "replay <run.json>",
		Short: "Mode 2: deterministic replay against fixtures",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			run, err := loadJSON[api.AgentRun](args[0])
			if err != nil {
				return err
			}

			provider, err := newFixtureProvider(nil, fixturesDir)
			if err != nil {
				return err
			}

			engine := replay.NewReplayEngine(provider)
			out, err := engine.ReplayTrace(context.Background(), run)
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			PrintReplayTerminal(&out)
			return nil
		},
	}
	cmd.Flags().StringVar(&fixturesDir, "fixtures", "", "directory of fixture JSON files")
	cmd.Flags().BoolVar(&asJSON, "json", false, "machine-readable JSON output")
	return cmd
}
