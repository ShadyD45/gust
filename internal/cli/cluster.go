package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"gust/internal/core/cluster"
	"gust/internal/core/scenario"
	"gust/pkg/api"
)

func newClusterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Failure mining and clustering",
	}
	cmd.AddCommand(newClusterRunsCmd())
	return cmd
}

func newClusterRunsCmd() *cobra.Command {
	var output string
	var asJSON bool
	var proposeDir string
	var limit int

	cmd := &cobra.Command{
		Use:   "runs <runs-dir>",
		Short: "Cluster AgentRun failures by deterministic fingerprints",
		Long: `Group near-duplicate buggy runs so continuous-eval proposers get one
representative failure per cluster. Fingerprints use tool sequence, error
codes, and argument schema shapes (not raw values).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runs, err := scenario.SampleRuns(args[0], limit)
			if err != nil {
				return err
			}
			if len(runs) == 0 {
				return fmt.Errorf("no valid AgentRun documents in %s", args[0])
			}
			rep, err := cluster.ClusterRuns(runs)
			if err != nil {
				return err
			}
			if proposeDir != "" {
				if err := os.MkdirAll(proposeDir, 0755); err != nil {
					return err
				}
				ext := scenario.NewExtractor()
				for _, c := range rep.Clusters {
					var run api.AgentRun
					for _, r := range runs {
						if r.RunID == c.Representative {
							run = r
							break
						}
					}
					if run.RunID == "" {
						continue
					}
					p, err := ext.ProposeFromRun(run, scenario.ProposeOptions{Redact: true})
					if err != nil {
						return err
					}
					cluster.AttachClusterID(p.Scenario, c.ID)
					dir := filepath.Join(proposeDir, c.ID)
					if err := writeScenarioDir(dir, p.Scenario); err != nil {
						return err
					}
				}
				fmt.Fprintf(os.Stderr, "wrote %d representative proposal(s) under %s\n", len(rep.Clusters), proposeDir)
			}
			if output != "" {
				raw, err := json.MarshalIndent(rep, "", "  ")
				if err != nil {
					return err
				}
				if err := os.WriteFile(output, append(raw, '\n'), 0644); err != nil {
					return err
				}
			}
			if asJSON || output == "" && proposeDir == "" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(rep)
			}
			fmt.Printf("clusters: %d from %d runs\n", len(rep.Clusters), rep.TotalRuns)
			for _, c := range rep.Clusters {
				fmt.Printf("  %s size=%d representative=%s\n", c.ID, c.Size, c.Representative)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "write cluster report JSON")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print cluster report as JSON")
	cmd.Flags().StringVar(&proposeDir, "propose-dir", "", "also write one redacted scenario proposal per cluster")
	cmd.Flags().IntVar(&limit, "limit", 0, "max runs to load (0 = all)")
	return cmd
}
