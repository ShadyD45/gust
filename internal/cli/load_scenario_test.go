package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveScenario_FixturesDirAndAssertionRef(t *testing.T) {
	root := t.TempDir()
	shared := filepath.Join(root, "_shared", "assertions")
	if err := os.MkdirAll(shared, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shared, "cancel.yaml"), []byte(`
- id: completes
  type: task_success
- id: cancels_right
  type: tool_call
  tool: cancel_order
  arguments:
    order_id: 123
`), 0644); err != nil {
		t.Fatal(err)
	}
	fxDir := filepath.Join(root, "cancel_latest", "fixtures")
	if err := os.MkdirAll(fxDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fxDir, "fx_get_orders_001.json"), []byte(`{
  "fixture_id": "fx_get_orders_001",
  "tool": "get_orders",
  "match_strategy": "exact_hash",
  "recorded_input": {"customer_id": 42},
  "recorded_response": {"status": "success", "body": [{"id": 122}]},
  "provenance": "recorded"
}`), 0644); err != nil {
		t.Fatal(err)
	}
	scPath := filepath.Join(root, "cancel_latest", "scenario.yaml")
	if err := os.WriteFile(scPath, []byte(`
id: cancel_latest
version: "1.0"
description: thin
task:
  id: t
  input: "Cancel my latest order"
environment:
  fixture_strategy: prefer_exact_then_sequence
  fixtures: []
  fixtures_dir: fixtures
assertions:
  - $ref: ../_shared/assertions/cancel.yaml
reliability:
  samples: 2
  minimum_pass_rate: 0.95
  confidence: 0.95
provenance:
  source: authored
  extracted_at: 2026-01-01T00:00:00Z
  reviewed_by: test
`), 0644); err != nil {
		t.Fatal(err)
	}

	sc, err := loadScenario(scPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(sc.Environment.Fixtures) != 1 || sc.Environment.Fixtures[0].FixtureID != "fx_get_orders_001" {
		t.Fatalf("fixtures %+v", sc.Environment.Fixtures)
	}
	if len(sc.Assertions) != 2 {
		t.Fatalf("assertions %d", len(sc.Assertions))
	}
	if sc.Assertions[0].ID != "completes" || sc.Assertions[1].ID != "cancels_right" {
		t.Fatalf("ids %+v", sc.Assertions)
	}
	if sc.Assertions[0].Ref != "" {
		t.Fatal("expected $ref stripped after resolve")
	}
}

func TestResolveScenario_DuplicateAssertionID(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pack.yaml"), []byte("- id: a\n  type: task_success\n"), 0644); err != nil {
		t.Fatal(err)
	}
	scPath := filepath.Join(root, "scenario.yaml")
	if err := os.WriteFile(scPath, []byte(`
id: dup
version: "1.0"
description: d
task: { id: t, input: "x" }
environment: { fixtures: [] }
assertion_files: [pack.yaml]
assertions:
  - id: a
    type: task_success
reliability: { samples: 1, minimum_pass_rate: 0.9, confidence: 0.95 }
provenance: { source: authored, extracted_at: 2026-01-01T00:00:00Z, reviewed_by: t }
`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadScenario(scPath); err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestResolveScenario_SiblingFixturesDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "fixtures"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "fixtures", "fx.json"), []byte(`{
  "fixture_id": "fx_sibling",
  "tool": "get_orders",
  "recorded_response": {"status": "success", "body": true},
  "provenance": "authored"
}`), 0644); err != nil {
		t.Fatal(err)
	}
	scPath := filepath.Join(root, "scenario.yaml")
	if err := os.WriteFile(scPath, []byte(`
id: sib
version: "1.0"
description: d
task: { id: t, input: "x" }
environment: { fixtures: [] }
assertions: [{ id: a, type: task_success }]
reliability: { samples: 1, minimum_pass_rate: 0.9, confidence: 0.95 }
provenance: { source: authored, extracted_at: 2026-01-01T00:00:00Z, reviewed_by: t }
`), 0644); err != nil {
		t.Fatal(err)
	}
	sc, err := loadScenario(scPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(sc.Environment.Fixtures) != 1 || sc.Environment.Fixtures[0].FixtureID != "fx_sibling" {
		t.Fatalf("%+v", sc.Environment.Fixtures)
	}
}

func TestDiscoverScenarioPaths(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "_shared", "assertions"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "_shared", "assertions", "x.yaml"), []byte("- id: a\n  type: task_success\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "one"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "one", "scenario.yaml"), []byte(`
id: one
version: "1.0"
description: d
task: { id: t, input: "x" }
environment: { fixtures: [] }
assertions: []
reliability: { samples: 1, minimum_pass_rate: 0.9, confidence: 0.95 }
provenance: { source: authored, extracted_at: 2026-01-01T00:00:00Z, reviewed_by: t }
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "loose.yaml"), []byte(`
id: loose
version: "1.0"
description: d
task: { id: t, input: "y" }
environment: { fixtures: [] }
assertions: []
reliability: { samples: 1, minimum_pass_rate: 0.9, confidence: 0.95 }
provenance: { source: authored, extracted_at: 2026-01-01T00:00:00Z, reviewed_by: t }
`), 0644); err != nil {
		t.Fatal(err)
	}

	paths, err := discoverScenarioPaths(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Fatalf("got %v", paths)
	}
}

func TestDiscoverTestdataSuites(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "suites")
	paths, err := discoverScenarioPaths(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) < 2 {
		t.Fatalf("expected at least 2 suite scenarios, got %v", paths)
	}
	for _, p := range paths {
		sc, err := loadScenario(p)
		if err != nil {
			t.Fatalf("load %s: %v", p, err)
		}
		if !looksLikeScenario(sc) {
			t.Fatalf("%s not a scenario after resolve", p)
		}
		if len(sc.Assertions) == 0 {
			t.Fatalf("%s expected resolved assertions", p)
		}
	}
}
