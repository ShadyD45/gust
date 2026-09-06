package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"gust/internal/adapters/fixtures"
	"gust/internal/adapters/ingest/otel"
	"gust/internal/core/policy"
	coretest "gust/internal/core/test"
	"gust/pkg/api"
)

func newTestCmd() *cobra.Command {
	var samples int
	var concurrency int
	var runnerName string
	var endpoint string
	var model string
	var policyPath string
	var passProb float64
	var asJSON bool
	var fixturesDir string
	var command string
	var traceSource string
	var tracePath string
	var otelListen string
	var otelGRPCListen string
	var timeoutSec int

	cmd := &cobra.Command{
		Use:   "test <scenario.yaml|dir> [-- command...]",
		Short: "Mode 3: probabilistic testing with Wilson reliability",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("scenario path or suite directory required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := discoverScenarioPaths(args[0])
			if err != nil {
				return fmt.Errorf("discover scenarios in %s: %w", args[0], err)
			}

			pol, err := loadPolicy(policyPath)
			if err != nil {
				return err
			}

			base := runnerFlags{
				Name:           runnerName,
				Endpoint:       endpoint,
				Model:          model,
				PassProb:       passProb,
				TraceSource:    traceSource,
				TracePath:      tracePath,
				OTelListen:     otelListen,
				OTelGRPCListen: otelGRPCListen,
				NameSet:        cmd.Flags().Changed("runner"),
				EndpointSet:    cmd.Flags().Changed("endpoint"),
			}
			if timeoutSec > 0 {
				base.Timeout = time.Duration(timeoutSec) * time.Second
			}
			if command != "" {
				base.Command = strings.Fields(command)
			}
			if len(args) > 1 {
				base.Command = args[1:]
			}

			provider := fixtures.NewMemoryFixtureProvider()
			proxy, err := fixtures.NewMockToolProxyServer(provider)
			if err != nil {
				return fmt.Errorf("start fixture proxy: %w", err)
			}
			fixtureEndpoint := proxy.Start()
			defer proxy.Close()

			var receiver *otel.Receiver
			ensureReceiver := func(flags runnerFlags) error {
				if receiver != nil || !shouldStartReceiver(flags) {
					return nil
				}
				listen := flags.OTelListen
				if listen == "" {
					listen = "127.0.0.1:0"
				}
				grpcListen := flags.OTelGRPCListen
				if grpcListen == "" {
					grpcListen = otel.DeriveGRPCListen(listen)
				}
				recv, err := otel.NewReceiver(otel.ReceiverConfig{
					Listen:     listen,
					GRPCListen: grpcListen,
				})
				if err != nil {
					return fmt.Errorf("start otel receiver: %w", err)
				}
				if err := recv.Start(); err != nil {
					return fmt.Errorf("start otel receiver: %w", err)
				}
				receiver = recv
				return nil
			}
			defer func() {
				if receiver != nil {
					_ = receiver.Close()
				}
			}()

			sampler := coretest.NewSampler(activeEvaluators())
			results := make([]*api.ReliabilityResult, 0, len(paths))

			for _, path := range paths {
				sc, err := loadScenario(path)
				if err != nil {
					return err
				}
				applyScenarioDefaults(&sc, samples)

				flags := mergeRunnerFlags(base, sc)
				if err := ensureReceiver(flags); err != nil {
					return err
				}
				if receiver != nil {
					flags.Receiver = receiver
					flags.OTelURL = receiver.Endpoint()
				}

				allFixtures := append([]api.Fixture{}, sc.Environment.Fixtures...)
				if extra, err := loadFixturesDir(fixturesDir); err == nil {
					allFixtures = append(allFixtures, extra...)
				}
				if err := provider.Replace(allFixtures); err != nil {
					return err
				}
				scConcurrency := concurrency
				if fixtures.HasOrderedFixtures(allFixtures) && scConcurrency > 1 {
					scConcurrency = 1
				}

				runner, err := resolveRunner(flags)
				if err != nil {
					return err
				}
				result, err := sampler.RunScenario(context.Background(), coretest.SamplingConfig{
					Scenario:        sc,
					Runner:          runner,
					Concurrency:     scConcurrency,
					Endpoint:        flags.Endpoint,
					FixtureEndpoint: fixtureEndpoint,
					FixtureProvider: provider,
					MinSamples:      pol.Reliability.MinSamplesForVerdict,
				})
				if err != nil {
					return fmt.Errorf("%s: %w", sc.ID, err)
				}
				results = append(results, result)
				if !asJSON {
					PrintReliabilityTerminal(result)
				}
			}

			eng := policy.NewEngine()
			verdict := eng.Evaluate(pol, results)

			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				payload := map[string]any{"policy": verdict}
				if len(results) == 1 {
					payload["reliability"] = results[0]
				} else {
					payload["reliability"] = results
				}
				if err := enc.Encode(payload); err != nil {
					return err
				}
			} else {
				if len(results) > 1 {
					PrintSuiteVerdict(len(results), verdict)
				}
				WriteGitHubSummary(results)
			}

			if verdict.ExitCode != api.ExitSuccess {
				os.Exit(verdict.ExitCode)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&samples, "samples", 0, "override scenario sample count")
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "parallel workers")
	cmd.Flags().StringVar(&runnerName, "runner", "synthetic", "test runner: synthetic|ollama|http|exec")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "agent URL (http) or Ollama URL (ollama)")
	cmd.Flags().StringVar(&model, "model", "llama3.1:8b", "ollama model")
	cmd.Flags().Float64Var(&passProb, "pass-probability", 1.0, "synthetic runner pass probability")
	cmd.Flags().StringVar(&policyPath, "policy", "", "policy YAML/JSON")
	cmd.Flags().BoolVar(&asJSON, "json", false, "machine-readable JSON output")
	cmd.Flags().StringVar(&fixturesDir, "fixtures", "", "extra fixture JSON directory overlaid on every scenario")
	cmd.Flags().StringVar(&command, "command", "", "exec runner command (or pass args after --)")
	cmd.Flags().StringVar(&traceSource, "trace-source", "", "how to collect the trace: auto|response|file|otel|otel-file")
	cmd.Flags().StringVar(&tracePath, "trace-path", "", "per-sample file path; may contain {sample_id}")
	cmd.Flags().StringVar(&otelListen, "otel-listen", "", "in-process OTLP/HTTP bind (default 127.0.0.1:0 when otel collection is on)")
	cmd.Flags().StringVar(&otelGRPCListen, "otel-grpc-listen", "", "in-process OTLP/gRPC bind (default derived from --otel-listen)")
	cmd.Flags().IntVar(&timeoutSec, "timeout", 0, "per-sample timeout in seconds")
	return cmd
}

func applyScenarioDefaults(sc *api.TestScenario, samples int) {
	if samples > 0 {
		sc.Reliability.Samples = samples
	}
	if sc.Reliability.Confidence == 0 {
		sc.Reliability.Confidence = 0.95
	}
	if sc.Reliability.MinimumPassRate == 0 {
		sc.Reliability.MinimumPassRate = 0.95
	}
	if len(sc.Assertions) == 0 {
		sc.Assertions = []api.Assertion{{ID: "default_success", Type: api.AssertTaskSuccess}}
	}
}

func shouldStartReceiver(flags runnerFlags) bool {
	if flags.TraceSource == "otel" || flags.OTelListen != "" {
		return true
	}
	src := strings.ToLower(flags.TraceSource)
	if src != "" && src != "auto" {
		return false
	}
	switch flags.Name {
	case "http", "exec":
		return true
	default:
		return false
	}
}
