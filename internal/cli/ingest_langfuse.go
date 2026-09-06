package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"gust/internal/adapters/ingest/langfuse"
)

func newIngestLangfuseCmd() *cobra.Command {
	var (
		host      string
		publicKey string
		secretKey string
		traceID   string
		sessionID string
		output    string
		opts      langfuse.Options
	)

	cmd := &cobra.Command{
		Use:   "langfuse",
		Short: "Pull a Langfuse trace or session into an AgentRun",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := langfuse.NewClient(langfuse.Config{
				Host:      host,
				PublicKey: publicKey,
				SecretKey: secretKey,
			})
			if err != nil {
				return err
			}

			id := traceID
			if id == "" && sessionID != "" {
				ids, err := client.ListSessionTraceIDs(sessionID)
				if err != nil {
					return err
				}
				if len(ids) == 0 {
					return fmt.Errorf("langfuse: session %s has no traces", sessionID)
				}
				if len(ids) > 1 {
					return fmt.Errorf("langfuse: session %s has multiple traces; pass --trace-id. ids: %v", sessionID, ids)
				}
				id = ids[0]
			}
			if id == "" {
				return fmt.Errorf("--trace-id or --session is required")
			}

			tr, err := client.FetchTrace(id)
			if err != nil {
				return err
			}
			run, err := langfuse.Map(tr, opts)
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

	cmd.Flags().StringVar(&host, "host", "", "Langfuse host (env LANGFUSE_HOST)")
	cmd.Flags().StringVar(&publicKey, "public-key", "", "prefer LANGFUSE_PUBLIC_KEY")
	cmd.Flags().StringVar(&secretKey, "secret-key", "", "prefer LANGFUSE_SECRET_KEY")
	cmd.Flags().StringVar(&traceID, "trace-id", "", "Langfuse trace id")
	cmd.Flags().StringVar(&sessionID, "session", "", "session id (must resolve to one trace unless --trace-id is set)")
	cmd.Flags().StringVar(&output, "output", "", "write AgentRun JSON (default stdout)")
	cmd.Flags().StringVar(&opts.RunID, "run-id", "", "override run id")
	cmd.Flags().StringVar(&opts.AgentName, "agent-name", "", "override agent name")
	cmd.Flags().StringVar(&opts.AgentVersion, "agent-version", "", "override agent version")
	cmd.Flags().StringVar(&opts.TaskID, "task-id", "", "override task id")
	cmd.Flags().StringVar(&opts.TaskInput, "task-input", "", "override task input")
	return cmd
}
