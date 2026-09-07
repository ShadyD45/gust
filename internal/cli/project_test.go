package cli

import (
	"os"
	"path/filepath"
	"testing"

	"gust/pkg/api"
)

func TestLoadProjectConfig(t *testing.T) {
	dir := t.TempDir()
	body := []byte(`policy: tests/policy.yaml
test:
  concurrency: 8
  timeout: 30s
retry:
  max_attempts: 3
  on: none
`)
	if err := os.WriteFile(filepath.Join(dir, "gust.yaml"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, path, err := loadProjectConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("expected gust.yaml path")
	}
	if cfg.Policy != "tests/policy.yaml" {
		t.Fatalf("policy=%q", cfg.Policy)
	}
	if cfg.Test.Concurrency != 8 {
		t.Fatalf("concurrency=%d", cfg.Test.Concurrency)
	}
	if cfg.Retry.MaxAttempts != 3 || cfg.Retry.On != api.RetryOnNone {
		t.Fatalf("retry=%+v", cfg.Retry)
	}
}

func TestLoadProjectConfigMissing(t *testing.T) {
	cfg, path, err := loadProjectConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Fatalf("expected no file, got %s", path)
	}
	if cfg.Retry.On != api.RetryOnTransient {
		t.Fatalf("default retry on=%s", cfg.Retry.On)
	}
}
