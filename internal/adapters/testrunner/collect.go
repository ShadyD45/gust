package testrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"gust/internal/adapters/ingest/otel"
	"gust/pkg/api"
)

const (
	EnvFixtureEndpoint = "AGENTEVAL_FIXTURE_ENDPOINT"
	EnvSampleID        = "AGENTEVAL_SAMPLE_ID"
	EnvOTelEndpoint    = "OTEL_EXPORTER_OTLP_ENDPOINT"
)

// TraceSource selects how a runner collects the AgentRun after invoking the agent.
type TraceSource string

const (
	TraceSourceAuto     TraceSource = "auto"
	TraceSourceResponse TraceSource = "response"
	TraceSourceFile     TraceSource = "file"
	TraceSourceOTelFile TraceSource = "otel-file"
	TraceSourceOTel     TraceSource = "otel"
)

// OTelWaiter is the in-process OTLP receiver used when Source is otel.
type OTelWaiter interface {
	Wait(ctx context.Context, sampleID string) (api.AgentRun, error)
}

// CollectorConfig controls post-invoke trace collection.
type CollectorConfig struct {
	Source       TraceSource
	PathTemplate string
	WaitTimeout  time.Duration
	Receiver     OTelWaiter
}

func (c CollectorConfig) source() TraceSource {
	if c.Source == "" {
		return TraceSourceAuto
	}
	return c.Source
}

func (c CollectorConfig) timeout() time.Duration {
	if c.WaitTimeout > 0 {
		return c.WaitTimeout
	}
	return 30 * time.Second
}

// ExpandPath substitutes {sample_id} in a path template.
func ExpandPath(template, sampleID string) string {
	return strings.ReplaceAll(template, "{sample_id}", sampleID)
}

// ParseAgentRun decodes and validates an AgentRun document.
func ParseAgentRun(data []byte) (api.AgentRun, error) {
	var run api.AgentRun
	if err := json.Unmarshal(data, &run); err != nil {
		return api.AgentRun{}, fmt.Errorf("decode agent run: %w", err)
	}
	if err := run.Validate(); err != nil {
		return api.AgentRun{}, fmt.Errorf("agent returned an invalid run: %w", err)
	}
	return run, nil
}

// Collect resolves an AgentRun from the invoke response, a filesystem path,
// an OTLP JSON file, or an in-process OTLP receiver.
func Collect(ctx context.Context, cfg CollectorConfig, sampleID string, responseBody []byte) (api.AgentRun, error) {
	src := cfg.source()
	switch src {
	case TraceSourceResponse:
		return ParseAgentRun(responseBody)
	case TraceSourceFile:
		return waitFileAgentRun(ctx, cfg, sampleID)
	case TraceSourceOTelFile:
		return waitFileOTel(ctx, cfg, sampleID)
	case TraceSourceOTel:
		return waitOTel(ctx, cfg, sampleID)
	case TraceSourceAuto:
		if run, err := ParseAgentRun(responseBody); err == nil {
			return run, nil
		}
		if cfg.PathTemplate != "" {
			if run, err := waitFileAgentRun(ctx, cfg, sampleID); err == nil {
				return run, nil
			}
			if run, err := waitFileOTel(ctx, cfg, sampleID); err == nil {
				return run, nil
			}
		}
		if cfg.Receiver != nil {
			return waitOTel(ctx, cfg, sampleID)
		}
		if len(responseBody) > 0 {
			return api.AgentRun{}, fmt.Errorf("auto collect: response was not a valid AgentRun and no file/otel source succeeded")
		}
		return api.AgentRun{}, fmt.Errorf("auto collect: no response body, file path, or otel receiver configured")
	default:
		return api.AgentRun{}, fmt.Errorf("unknown trace source %q", src)
	}
}

func waitFileAgentRun(ctx context.Context, cfg CollectorConfig, sampleID string) (api.AgentRun, error) {
	data, err := waitFile(ctx, ExpandPath(cfg.PathTemplate, sampleID), cfg.timeout())
	if err != nil {
		return api.AgentRun{}, err
	}
	return ParseAgentRun(data)
}

func waitFileOTel(ctx context.Context, cfg CollectorConfig, sampleID string) (api.AgentRun, error) {
	data, err := waitFile(ctx, ExpandPath(cfg.PathTemplate, sampleID), cfg.timeout())
	if err != nil {
		return api.AgentRun{}, err
	}
	opts := otel.Options{RunID: sampleID}
	if sampleID != "" {
		opts.TraceID = ""
	}
	run, err := otel.NewMapper().MapBytes(data, opts)
	if err != nil {
		return api.AgentRun{}, err
	}
	if run.Metadata == nil {
		run.Metadata = map[string]any{}
	}
	run.Metadata["sample_id"] = sampleID
	return run, nil
}

func waitOTel(ctx context.Context, cfg CollectorConfig, sampleID string) (api.AgentRun, error) {
	if cfg.Receiver == nil {
		return api.AgentRun{}, fmt.Errorf("otel collect: no receiver configured; pass --otel-listen")
	}
	waitCtx, cancel := context.WithTimeout(ctx, cfg.timeout())
	defer cancel()
	run, err := cfg.Receiver.Wait(waitCtx, sampleID)
	if err != nil {
		return api.AgentRun{}, err
	}
	if err := run.Validate(); err != nil {
		return api.AgentRun{}, fmt.Errorf("otel receiver returned an invalid run: %w", err)
	}
	return run, nil
}

func waitFile(ctx context.Context, path string, timeout time.Duration) ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("trace path is empty")
	}
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			return data, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for trace file %s", path)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
