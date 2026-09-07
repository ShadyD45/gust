package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"gust/internal/adapters/wire"
	"gust/pkg/api"
)

// ProjectConfig is the gust.yaml project file.
type ProjectConfig struct {
	Policy  string            `yaml:"policy"`
	Test    ProjectTestConfig `yaml:"test"`
	Retry   api.RetryPolicy   `yaml:"retry"`
	Wire    ProjectWireConfig `yaml:"wire"`
	Plugins []ProjectPlugin   `yaml:"plugins"`
}

// ProjectTestConfig holds Mode 3 execution defaults.
type ProjectTestConfig struct {
	Concurrency int    `yaml:"concurrency"`
	Timeout     string `yaml:"timeout"`
	Samples     int    `yaml:"samples"`
}

// ProjectWireConfig holds Tier-2 plugin timeouts.
type ProjectWireConfig struct {
	EvaluateTimeout  string `yaml:"evaluate_timeout"`
	HandshakeTimeout string `yaml:"handshake_timeout"`
}

func defaultProjectConfig() ProjectConfig {
	return ProjectConfig{
		Retry: api.RetryPolicy{
			MaxAttempts: api.DefaultRetryMaxAttempts,
			BackoffMs:   api.DefaultRetryBackoffMs,
			On:          api.RetryOnTransient,
		},
		Test: ProjectTestConfig{Concurrency: 4},
	}
}

// loadProjectConfig walks from start (or cwd) toward root for gust.yaml.
func loadProjectConfig(start string) (ProjectConfig, string, error) {
	cfg := defaultProjectConfig()
	if start == "" {
		wd, err := os.Getwd()
		if err != nil {
			return cfg, "", err
		}
		start = wd
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return cfg, "", err
	}
	for {
		path := filepath.Join(dir, "gust.yaml")
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			loaded, err := parseProjectFile(path)
			if err != nil {
				return cfg, path, err
			}
			merged := mergeProject(cfg, loaded)
			return merged, path, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return cfg, "", nil
		}
		dir = parent
	}
}

func parseProjectFile(path string) (ProjectConfig, error) {
	var cfg ProjectConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

func mergeProject(base, over ProjectConfig) ProjectConfig {
	if over.Policy != "" {
		base.Policy = over.Policy
	}
	if over.Test.Concurrency > 0 {
		base.Test.Concurrency = over.Test.Concurrency
	}
	if over.Test.Timeout != "" {
		base.Test.Timeout = over.Test.Timeout
	}
	if over.Test.Samples > 0 {
		base.Test.Samples = over.Test.Samples
	}
	base.Retry = overlayRetry(base.Retry, over.Retry)
	if over.Wire.EvaluateTimeout != "" {
		base.Wire.EvaluateTimeout = over.Wire.EvaluateTimeout
	}
	if over.Wire.HandshakeTimeout != "" {
		base.Wire.HandshakeTimeout = over.Wire.HandshakeTimeout
	}
	if len(over.Plugins) > 0 {
		base.Plugins = over.Plugins
	}
	return base
}

func overlayRetry(base, over api.RetryPolicy) api.RetryPolicy {
	if over.MaxAttempts > 0 {
		base.MaxAttempts = over.MaxAttempts
	}
	if over.BackoffMs > 0 {
		base.BackoffMs = over.BackoffMs
	}
	if over.On != "" {
		base.On = over.On
	}
	return base
}

func applyWireTimeouts(cfg ProjectConfig) {
	var handshake, evaluate time.Duration
	if cfg.Wire.HandshakeTimeout != "" {
		if d, err := time.ParseDuration(cfg.Wire.HandshakeTimeout); err == nil {
			handshake = d
		}
	}
	if cfg.Wire.EvaluateTimeout != "" {
		if d, err := time.ParseDuration(cfg.Wire.EvaluateTimeout); err == nil {
			evaluate = d
		}
	}
	wire.SetTimeouts(handshake, evaluate)
}

func parseOptionalDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}
