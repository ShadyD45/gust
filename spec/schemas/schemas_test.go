package schemas_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestSchemaFilesPresent verifies Draft 2020-12 contract schemas ship with the repo.
// Runtime validation remains hand-rolled Validate() for MVP; schemas are the public contract.
func TestSchemaFilesPresent(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	names := []string{
		"agent_run.json",
		"evaluation_result.json",
		"fixture.json",
		"policy.json",
		"reliability_result.json",
		"test_scenario.json",
	}
	for _, name := range names {
		path := filepath.Join(root, "spec", "schemas", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("missing schema %s: %v", name, err)
		}
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatalf("invalid JSON in %s: %v", name, err)
		}
		schema, _ := doc["$schema"].(string)
		if schema == "" {
			t.Errorf("%s missing $schema", name)
		}
	}
}
