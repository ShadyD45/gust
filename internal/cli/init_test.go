package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScaffoldProject(t *testing.T) {
	dir := t.TempDir()
	if err := scaffoldProject(dir); err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	want := []string{
		"gust.yaml",
		"tests/policy.yaml",
		"tests/scenarios/example.yaml",
		"tests/fixtures/.gitkeep",
		"tests/assertions/.gitkeep",
	}
	for _, rel := range want {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}

	// Idempotent: second run should not error
	if err := scaffoldProject(dir); err != nil {
		t.Fatalf("second scaffold: %v", err)
	}
}
