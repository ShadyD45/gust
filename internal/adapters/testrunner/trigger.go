package testrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

var _ ports.TestRunner = (*TriggerRunner)(nil)

// TriggerRunner launches a user-owned harness command and optionally fetches the
// resulting trace from an existing backend via TraceFetchCommand.
//
// This is the no-agent-code remote QA path: the harness owns setup/mocks/invocation;
// Gust supplies sample/trace context and evaluates the retrieved AgentRun.
type TriggerRunner struct {
	Argv               []string
	TraceFetchCommand  []string
	Collector          CollectorConfig
	Timeout            time.Duration
	FetchPollInterval  time.Duration
	FetchTimeout       time.Duration
}

// NewTriggerRunner constructs a harness-driven sample runner.
func NewTriggerRunner(argv []string, collector CollectorConfig) *TriggerRunner {
	timeout := collector.timeout()
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &TriggerRunner{
		Argv:              argv,
		Collector:         collector,
		Timeout:           timeout,
		FetchPollInterval: 500 * time.Millisecond,
		FetchTimeout:      timeout,
	}
}

func (r *TriggerRunner) Name() string { return "trigger" }

func (r *TriggerRunner) Run(ctx context.Context, req ports.SampleRequest) (api.AgentRun, error) {
	if len(r.Argv) == 0 {
		return api.AgentRun{}, fmt.Errorf("trigger runner: command is required")
	}
	req = normalizeSampleRequest(req, req.OTelEndpoint)

	payload, err := json.Marshal(map[string]any{
		"evaluation_id": req.EvaluationID,
		"scenario_id":   req.ScenarioID,
		"sample_id":     req.SampleID,
		"trace_id":      req.TraceID,
		"traceparent":   req.TraceParent,
		"baggage":       req.Baggage,
		"input":         req.Scenario.Task.Input,
		"context":       req.Scenario.Task.Context,
		"task":          req.Scenario.Task,
		"world_control": string(req.WorldMode),
		"tool_endpoint": req.FixtureEndpointOrEmpty(),
		"otel_endpoint": req.OTelEndpoint,
		"ingest_url":    req.IngestURL,
	})
	if err != nil {
		return api.AgentRun{}, err
	}

	runCtx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, r.Argv[0], r.Argv[1:]...)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = applySampleEnv(append([]string{}, os.Environ()...), req)

	if err := cmd.Run(); err != nil {
		return api.AgentRun{}, fmt.Errorf("trigger %v: %w\nstderr: %s", r.Argv, err, truncate(stderr.Bytes(), 1024))
	}

	receipt, hasReceipt := parseExecutionReceipt(stdout.Bytes())
	if hasReceipt && receipt.Run != nil {
		run := *receipt.Run
		stampSampleMetadata(&run, req)
		return run, nil
	}

	traceID := req.TraceID
	if hasReceipt && receipt.TraceID != "" {
		traceID = receipt.TraceID
	}

	if len(r.TraceFetchCommand) > 0 {
		run, err := r.fetchTrace(runCtx, req, traceID)
		if err != nil {
			return api.AgentRun{}, err
		}
		stampSampleMetadata(&run, req)
		return run, nil
	}

	run, err := Collect(runCtx, r.Collector, req.SampleID, stdout.Bytes())
	if err != nil {
		return api.AgentRun{}, err
	}
	stampSampleMetadata(&run, req)
	return run, nil
}

func (r *TriggerRunner) fetchTrace(ctx context.Context, req ports.SampleRequest, traceID string) (api.AgentRun, error) {
	deadline := time.Now().Add(r.FetchTimeout)
	interval := r.FetchPollInterval
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	var lastErr error
	for {
		argv := expandTraceFetchArgv(r.TraceFetchCommand, req, traceID)
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = applySampleEnv(append([]string{}, os.Environ()...), req)
		if err := cmd.Run(); err != nil {
			lastErr = fmt.Errorf("trace fetch %v: %w\nstderr: %s", argv, err, truncate(stderr.Bytes(), 512))
		} else if run, err := ParseAgentRun(stdout.Bytes()); err == nil {
			return run, nil
		} else if mapped, mapErr := tryParseOTLP(stdout.Bytes(), req.SampleID, traceID); mapErr == nil {
			return mapped, nil
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				return api.AgentRun{}, fmt.Errorf("trace fetch timed out for %s: %w", traceID, lastErr)
			}
			return api.AgentRun{}, fmt.Errorf("trace fetch timed out for %s", traceID)
		}
		select {
		case <-ctx.Done():
			return api.AgentRun{}, ctx.Err()
		case <-time.After(interval):
		}
	}
}

func expandTraceFetchArgv(argv []string, req ports.SampleRequest, traceID string) []string {
	out := make([]string, len(argv))
	replacer := strings.NewReplacer(
		"{trace_id}", traceID,
		"{sample_id}", req.SampleID,
		"{evaluation_id}", req.EvaluationID,
		"{scenario_id}", req.ScenarioID,
	)
	for i, a := range argv {
		out[i] = replacer.Replace(a)
	}
	return out
}

func parseExecutionReceipt(data []byte) (ports.ExecutionReceipt, bool) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return ports.ExecutionReceipt{}, false
	}
	var receipt ports.ExecutionReceipt
	if err := json.Unmarshal(trimmed, &receipt); err != nil {
		return ports.ExecutionReceipt{}, false
	}
	if receipt.Status == "" && receipt.TraceID == "" && receipt.RunID == "" && receipt.Run == nil {
		return ports.ExecutionReceipt{}, false
	}
	return receipt, true
}
