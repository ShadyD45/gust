package cli

import (
	"fmt"
	"strings"
	"time"

	"gust/internal/adapters/testrunner"
	"gust/internal/ports"
	"gust/internal/registry"
	"gust/pkg/api"
)

// runnerFlags is the merged CLI + scenario configuration for Mode 3.
type runnerFlags struct {
	Name           string
	Endpoint       string
	Model          string
	PassProb       float64
	Command        []string
	TraceSource    string
	TracePath      string
	OTelListen     string
	OTelGRPCListen string
	Timeout        time.Duration
	WaitTimeout    time.Duration
	Receiver       testrunner.OTelWaiter
	OTelURL        string
	EndpointSet    bool
	NameSet        bool
}

func mergeRunnerFlags(flags runnerFlags, sc api.TestScenario) runnerFlags {
	if sc.Runner == nil {
		return flags
	}
	spec := sc.Runner
	if !flags.NameSet && spec.Type != "" {
		flags.Name = spec.Type
	}
	if spec.URL != "" && (flags.Endpoint == "" || !flags.EndpointSet) {
		flags.Endpoint = spec.URL
	}
	if len(flags.Command) == 0 && len(spec.Command) > 0 {
		flags.Command = spec.Command
	}
	if spec.TimeoutSeconds > 0 && flags.Timeout == 0 {
		flags.Timeout = time.Duration(spec.TimeoutSeconds) * time.Second
	}
	if flags.TraceSource == "" && spec.Traces.Source != "" {
		flags.TraceSource = spec.Traces.Source
	}
	if flags.TracePath == "" && spec.Traces.Path != "" {
		flags.TracePath = spec.Traces.Path
	}
	if spec.Traces.WaitTimeoutSeconds > 0 && flags.WaitTimeout == 0 {
		flags.WaitTimeout = time.Duration(spec.Traces.WaitTimeoutSeconds) * time.Second
	}
	return flags
}

func collectorFromFlags(flags runnerFlags) testrunner.CollectorConfig {
	src := testrunner.TraceSource(strings.ToLower(flags.TraceSource))
	wait := flags.WaitTimeout
	if wait == 0 {
		wait = flags.Timeout
	}
	return testrunner.CollectorConfig{
		Source:       src,
		PathTemplate: flags.TracePath,
		WaitTimeout:  wait,
		Receiver:     flags.Receiver,
	}
}

func resolveRunner(flags runnerFlags) (ports.TestRunner, error) {
	name := flags.Name
	if name == "" {
		name = "synthetic"
	}
	switch name {
	case "synthetic":
		return testrunner.NewSyntheticRunner(flags.PassProb, 42), nil
	case "ollama":
		endpoint := flags.Endpoint
		if endpoint == "" {
			endpoint = "http://localhost:11434"
		}
		return testrunner.NewOllamaRunner(endpoint, flags.Model), nil
	case "http":
		if flags.Endpoint == "" {
			return nil, fmt.Errorf("http runner requires --endpoint or scenario.runner.url")
		}
		r := testrunner.NewHTTPRunner(flags.Endpoint, collectorFromFlags(flags))
		if flags.OTelURL != "" {
			r.WithOTelURL(flags.OTelURL)
		}
		if flags.Timeout > 0 {
			r.Timeout = flags.Timeout
		}
		return r, nil
	case "exec":
		if len(flags.Command) == 0 {
			return nil, fmt.Errorf("exec runner requires a command (--command or args after --)")
		}
		r := testrunner.NewExecRunner(flags.Command, collectorFromFlags(flags))
		if flags.OTelURL != "" {
			r.WithOTelURL(flags.OTelURL)
		}
		if flags.Timeout > 0 {
			r.Timeout = flags.Timeout
		}
		return r, nil
	default:
		if tr, ok := registry.DefaultRegistry.GetTestRunner(name); ok {
			return tr, nil
		}
		return nil, fmt.Errorf("unknown runner %q", name)
	}
}
