package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"

	"gust/internal/adapters/ingest/otel"
	"gust/internal/core/analyze"
	"gust/internal/ports"
	"gust/pkg/api"
)

func newIngestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ingest",
		Short: "Convert external traces into gust AgentRun documents",
	}
	cmd.AddCommand(newIngestOtelCmd())
	cmd.AddCommand(newIngestLangfuseCmd())
	return cmd
}

func newIngestOtelCmd() *cobra.Command {
	var (
		file     string
		url      string
		output   string
		listOnly bool
		opts     otel.Options
	)

	cmd := &cobra.Command{
		Use:   "otel",
		Short: "Map an OTLP export (file, stdin, URL, or live serve) to an AgentRun",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readIngestBytes(file, url)
			if err != nil {
				return err
			}

			if listOnly {
				var payload otel.ExportPayload
				if err := json.Unmarshal(data, &payload); err != nil {
					return fmt.Errorf("parse OTLP export: %w", err)
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

	cmd.Flags().StringVar(&file, "file", "", "OTLP JSON export file (`-` reads stdin)")
	cmd.Flags().StringVar(&url, "url", "", "HTTP(S) URL of an OTLP JSON export")
	cmd.Flags().StringVar(&output, "output", "", "write AgentRun JSON to file (default stdout)")
	cmd.Flags().BoolVar(&listOnly, "list-traces", false, "list trace ids in the export and exit")
	cmd.Flags().StringVar(&opts.TraceID, "trace-id", "", "trace to map when the export contains several")
	cmd.Flags().StringVar(&opts.RunID, "run-id", "", "override run id (defaults to trace id)")
	cmd.Flags().StringVar(&opts.AgentName, "agent-name", "", "override resource service.name")
	cmd.Flags().StringVar(&opts.AgentVersion, "agent-version", "", "override resource service.version")
	cmd.Flags().StringVar(&opts.TaskID, "task-id", "", "override derived task id")
	cmd.Flags().StringVar(&opts.TaskInput, "task-input", "", "override derived task input")
	cmd.AddCommand(newIngestOtelServeCmd())
	return cmd
}

func newIngestOtelServeCmd() *cobra.Command {
	var listen string
	var grpcListen string
	var outputDir string
	var stdout bool
	var doAnalyze bool
	var assertionsPath string
	var policyPath string
	var judgePlugin string
	var plugins []string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Listen for live OTLP and AgentRun exports (HTTP + gRPC)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			writeDir := outputDir
			if writeDir == "" && !stdout && !doAnalyze {
				writeDir = "runs"
			}
			if writeDir != "" {
				if err := os.MkdirAll(writeDir, 0755); err != nil {
					return err
				}
			}

			var assertions []api.Assertion
			if doAnalyze {
				cleanup, err := loadPluginsForCommand(plugins, judgePlugin)
				if err != nil {
					return err
				}
				defer cleanup()
				var errAssert error
				assertions, errAssert = loadAssertions(api.AgentRun{}, assertionsPath)
				if errAssert != nil {
					return errAssert
				}
				if _, err := loadPolicy(policyPath); err != nil {
					return err
				}
			}

			var failed atomic.Bool
			engine := analyze.NewEngine(activeEvaluators())
			recv, err := otel.NewReceiver(otel.ReceiverConfig{
				Listen:     listen,
				GRPCListen: grpcListen,
				OnRun: func(run api.AgentRun) error {
					if writeDir != "" {
						encoded, err := json.MarshalIndent(run, "", "  ")
						if err != nil {
							return err
						}
						path := filepath.Join(writeDir, run.RunID+".json")
						if err := os.WriteFile(path, append(encoded, '\n'), 0644); err != nil {
							return err
						}
					}
					if stdout {
						encoded, err := json.Marshal(run)
						if err != nil {
							return err
						}
						_, _ = os.Stdout.Write(append(encoded, '\n'))
					}
					if doAnalyze {
						report, err := engine.AnalyzeRun(context.Background(), run, assertions, ports.EvaluationContext{
							ScenarioID: run.RunID,
						})
						if err != nil {
							return err
						}
						PrintAnalyzeTerminal(report)
						if !report.Passed {
							failed.Store(true)
						}
					}
					return nil
				},
			})
			if err != nil {
				return err
			}
			if err := recv.Start(); err != nil {
				return err
			}
			defer recv.Close()
			fmt.Fprintf(os.Stderr, "gust ingest otel serve OTLP/HTTP %s (POST /v1/traces, POST /v1/runs)\n", recv.Endpoint())
			if grpc := recv.GRPCEndpoint(); grpc != "" {
				fmt.Fprintf(os.Stderr, "OTLP/gRPC %s\n", grpc)
			}
			<-cmd.Context().Done()
			if failed.Load() {
				os.Exit(api.ExitFailure)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&listen, "listen", ":4318", "OTLP/HTTP bind address")
	cmd.Flags().StringVar(&grpcListen, "grpc-listen", ":4317", "OTLP/gRPC bind address")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "directory for completed AgentRun JSON (default runs if not --stdout/--analyze)")
	cmd.Flags().BoolVar(&stdout, "stdout", false, "print each mapped AgentRun as JSON")
	cmd.Flags().BoolVar(&doAnalyze, "analyze", false, "run assertions against each ingested run")
	cmd.Flags().StringVar(&assertionsPath, "assertions", "", "assertions JSON used with --analyze")
	cmd.Flags().StringVar(&policyPath, "policy", "", "policy YAML/JSON used with --analyze")
	cmd.Flags().StringVar(&judgePlugin, "judge-plugin", "", "Tier-2 LLM judge plugin (deprecated alias of --plugin)")
	cmd.Flags().StringArrayVar(&plugins, "plugin", nil, "Tier-2 evaluator plugin; repeatable")
	return cmd
}

func readIngestBytes(file, url string) ([]byte, error) {
	if url != "" {
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", url, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
		}
		return io.ReadAll(resp.Body)
	}
	if file == "" {
		stat, err := os.Stdin.Stat()
		if err == nil && stat.Mode()&os.ModeCharDevice == 0 {
			file = "-"
		} else {
			return nil, fmt.Errorf("--file, --url, or piped stdin is required")
		}
	}
	if file == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(file)
}
