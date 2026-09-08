package testrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

var _ ports.TestRunner = (*ExecRunner)(nil)

// ExecRunner starts a user-owned process once per sample and collects the trace.
type ExecRunner struct {
	Argv      []string
	Collector CollectorConfig
	OTelURL   string
	Timeout   time.Duration
}

// NewExecRunner constructs a runner that execs argv per sample.
func NewExecRunner(argv []string, collector CollectorConfig) *ExecRunner {
	timeout := collector.timeout()
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &ExecRunner{Argv: argv, Collector: collector, Timeout: timeout}
}

func (r *ExecRunner) Name() string { return "exec" }

func (r *ExecRunner) WithOTelURL(url string) *ExecRunner {
	r.OTelURL = url
	return r
}

func (r *ExecRunner) Run(ctx context.Context, req ports.SampleRequest) (api.AgentRun, error) {
	if len(r.Argv) == 0 {
		return api.AgentRun{}, fmt.Errorf("exec runner: command is required")
	}
	req = normalizeSampleRequest(req, r.OTelURL)

	payload, err := json.Marshal(map[string]any{
		"id":             req.Scenario.ID,
		"evaluation_id":  req.EvaluationID,
		"scenario_id":    req.ScenarioID,
		"task":           req.Scenario.Task,
		"input":          req.Scenario.Task.Input,
		"context":        req.Scenario.Task.Context,
		"tool_endpoint":  req.FixtureEndpointOrEmpty(),
		"sample_id":      req.SampleID,
		"trace_id":       req.TraceID,
		"traceparent":    req.TraceParent,
		"baggage":        req.Baggage,
		"otel_endpoint":  req.OTelEndpoint,
		"ingest_url":     req.IngestURL,
		"world_control":  string(req.WorldMode),
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
		return api.AgentRun{}, fmt.Errorf("exec %v: %w\nstderr: %s", r.Argv, err, truncate(stderr.Bytes(), 1024))
	}

	run, err := Collect(runCtx, r.Collector, req.SampleID, stdout.Bytes())
	if err != nil {
		return api.AgentRun{}, err
	}
	stampSampleMetadata(&run, req)
	return run, nil
}
