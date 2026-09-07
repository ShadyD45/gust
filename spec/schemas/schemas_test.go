package schemas_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gust/pkg/api"
)

// Contract split (documented for contributors):
//   - JSON Schema in this package = wire/public contract for artifacts
//   - Go Validate() methods = semantic invariants beyond structural shape
// Both should stay aligned for assertion type enums and similar closed sets.

func schemaRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// TestSchemaFilesPresent verifies Draft 2020-12 contract schemas ship with the repo.
// Runtime validation remains hand-rolled Validate() for MVP; schemas are the public contract.
func TestSchemaFilesPresent(t *testing.T) {
	root := schemaRoot(t)
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

func TestAssertionTypesMatchSchemaEnum(t *testing.T) {
	root := schemaRoot(t)
	path := filepath.Join(root, "spec", "schemas", "test_scenario.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	props := doc["properties"].(map[string]any)
	assertions := props["assertions"].(map[string]any)
	items := assertions["items"].(map[string]any)
	itemProps := items["properties"].(map[string]any)
	typeField := itemProps["type"].(map[string]any)
	enumRaw := typeField["enum"].([]any)

	schemaTypes := make(map[string]bool, len(enumRaw))
	for _, v := range enumRaw {
		schemaTypes[v.(string)] = true
	}

	goTypes := []api.AssertionType{
		api.AssertTaskSuccess,
		api.AssertToolCall,
		api.AssertForbiddenToolCall,
		api.AssertRequiredTool,
		api.AssertToolSequence,
		api.AssertMaxSteps,
		api.AssertMaxLatency,
		api.AssertSchemaValid,
		api.AssertErrorRecovery,
		api.AssertLLMJudge,
	}
	for _, at := range goTypes {
		if !schemaTypes[string(at)] {
			t.Errorf("Go AssertionType %q missing from test_scenario.json enum", at)
		}
	}
	for st := range schemaTypes {
		found := false
		for _, at := range goTypes {
			if string(at) == st {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("schema enum value %q has no matching Go AssertionType", st)
		}
	}
}
