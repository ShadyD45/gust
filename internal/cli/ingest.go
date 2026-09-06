package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"gust/internal/adapters/ingest/otel"
)

func newIngestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ingest",
		Short: "Convert external trace formats into gust AgentRun files",
	}
	cmd.AddCommand(newIngestOtelCmd())
	return cmd
}

func newIngestOtelCmd() *cobra.Command {
	var (
		file     string
		output   string
		listOnly bool
		opts     otel.Options
	)

	cmd := &cobra.Command{
		Use:   "otel",
		Short: "Map an OTLP JSON export (OpenInference conventions) to an AgentRun",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file is required")
			}
			data, err := os.ReadFile(file)
			if err != nil {
				return err
			}

			if listOnly {
				var payload otel.ExportPayload
				if err := json.Unmarshal(data, &payload); err != nil {
					return fmt.Errorf("parse %s: %w", file, err)
				}
				for _, id := range otel.TraceIDs(payload) {
					fmt.Println(id)
				}
				return nil
			}

			run, err := otel.NewMapper().MapBytes(data, opts)
			if err != nil {
				return err
			}

			encoded, err := json.MarshalIndent(run, "", "  ")
			if err != nil {
				return err
			}
			encoded = append(encoded, '\n')

			if output == "" {
				_, err := os.Stdout.Write(encoded)
				return err
			}
			if dir := filepath.Dir(output); dir != "." {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return err
				}
			}
			return os.WriteFile(output, encoded, 0644)
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "OTLP JSON export file")
	cmd.Flags().StringVar(&output, "output", "", "write AgentRun JSON to file (default stdout)")
	cmd.Flags().BoolVar(&listOnly, "list-traces", false, "list trace ids in the export and exit")
	cmd.Flags().StringVar(&opts.TraceID, "trace-id", "", "trace to map when the export contains several")
	cmd.Flags().StringVar(&opts.RunID, "run-id", "", "override run id (defaults to trace id)")
	cmd.Flags().StringVar(&opts.AgentName, "agent-name", "", "override resource service.name")
	cmd.Flags().StringVar(&opts.AgentVersion, "agent-version", "", "override resource service.version")
	cmd.Flags().StringVar(&opts.TaskID, "task-id", "", "override derived task id")
	cmd.Flags().StringVar(&opts.TaskInput, "task-input", "", "override derived task input")
	return cmd
}
