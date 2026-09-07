package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"gust/internal/core/dataset"
	"gust/pkg/api"
)

func newDatasetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dataset",
		Short: "Content-addressed scenario dataset bundle and verify",
	}
	cmd.AddCommand(newDatasetBundleCmd(), newDatasetVerifyCmd())
	return cmd
}

func newDatasetBundleCmd() *cobra.Command {
	var id string
	var from string
	var force bool
	cmd := &cobra.Command{
		Use:   "bundle <dir>",
		Short: "Write scenario JSON files and an immutable dataset.json manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(id) == "" {
				return fmt.Errorf("--id is required")
			}
			dir := args[0]
			src := from
			if src == "" {
				src = dir
			}
			paths, err := discoverScenarioPaths(src)
			if err != nil {
				return err
			}
			if len(paths) == 0 {
				return fmt.Errorf("no scenarios found in %s", src)
			}
			scenarios := make([]api.TestScenario, 0, len(paths))
			for _, p := range paths {
				if strings.EqualFold(filepath.Base(p), "dataset.json") {
					continue
				}
				sc, err := loadScenario(p)
				if err != nil {
					return err
				}
				scenarios = append(scenarios, sc)
			}
			m, err := dataset.Bundle(dir, id, scenarios, force)
			if err != nil {
				return err
			}
			fmt.Printf("bundled %d scenarios id=%s hash=%s\n", len(m.ScenarioIDs), m.ID, m.ContentHash)
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "dataset identifier")
	cmd.Flags().StringVar(&from, "from", "", "scenario file or directory (default: <dir>)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite changed scenarios and prune orphan JSON files")
	return cmd
}

func newDatasetVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify <dir>",
		Short: "Verify on-disk scenarios still match dataset.json hashes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := dataset.Verify(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "ok: %s\n", args[0])
			return nil
		},
	}
}
