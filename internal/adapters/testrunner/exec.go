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

func (r *ExecRunner) Run(ctx context.Context, scenario api.TestScenario, fixtureEndpoint string) (api.AgentRun, error) {
	if fixtureEndpoint == "" {
		fixtureEndpoint = os.Getenv(EnvFixtureEndpoint)
	}
	if len(r.Argv) == 0 {
		return api.AgentRun{}, fmt.Errorf("exec runner: command is required")
	}

	sampleID := newSampleID(scenario.ID)
	payload, err := json.Marshal(map[string]any{
		"id":            scenario.ID,
		"task":          scenario.Task,
		"input":         scenario.Task.Input,
		"context":       scenario.Task.Context,
		"tool_endpoint": fixtureEndpoint,
		"sample_id":     sampleID,
		"otel_endpoint": r.OTelURL,
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
	cmd.Env = append(os.Environ(),
		EnvFixtureEndpoint+"="+fixtureEndpoint,
		EnvSampleID+"="+sampleID,
	)
	cmd.Env = ApplyOTelEnv(cmd.Env, r.OTelURL, sampleID)

	if err := cmd.Run(); err != nil {
		return api.AgentRun{}, fmt.Errorf("exec %v: %w\nstderr: %s", r.Argv, err, truncate(stderr.Bytes(), 1024))
	}

	run, err := Collect(runCtx, r.Collector, sampleID, stdout.Bytes())
	if err != nil {
		return api.AgentRun{}, err
	}
	stampSampleMetadata(&run, sampleID)
	return run, nil
}
