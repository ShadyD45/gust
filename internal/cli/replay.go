package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"gust/internal/adapters/fixtures"
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

			provider := fixtures.NewMemoryFixtureProvider()
			if fixturesDir != "" {
				entries, err := os.ReadDir(fixturesDir)
				if err != nil {
					return err
				}
				var loaded []api.Fixture
				for _, e := range entries {
					if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
						continue
					}
					fx, err := loadJSON[api.Fixture](filepath.Join(fixturesDir, e.Name()))
					if err != nil {
						return err
					}
					loaded = append(loaded, fx)
				}
				if err := provider.LoadFixtures(loaded); err != nil {
					return err
				}
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
