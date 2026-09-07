# Phase 8: Assisted Scenario Extraction & Dataset Management

## 1. Objectives & Scope
1. Implement the **Scenario Extraction Tooling** (`gust scenario from-run`) converting captured production/staging `AgentRun` traces into reusable `TestScenario` YAML files.
2. Structurally enforce **Hypothesis H8**: Guarantee that extracted scenarios **never** auto-populate `assertions` from observed trace behavior, safeguarding against enshrining bugs as passing tests.
3. Automatically extract tool calls as candidate `Fixture`s with `provenance: "recorded"`.
4. Implement content-addressed `Dataset` bundling for reproducible test suites.

---

## 2. Package Architecture

```text
gust/
├── internal/
│   └── core/
│       ├── scenario/            # Scenario Extraction Workflow
│       │   ├── extractor.go     # Extraction logic & structural invariant enforcement
│       │   ├── template.go      # YAML template emitter
│       │   └── extractor_test.go
│       └── dataset/             # Content-Addressed Datasets
│           ├── dataset.go
│           └── dataset_test.go
```

---

## 3. Extraction Engine & Safety Invariants (`internal/core/scenario/`)

### 3.1 Structural Invariant Enforcement (Hypothesis H8)
```go
package scenario

import (
    "time"
    "gust/pkg/api"
    "gust/pkg/jcs"
)

type Extractor struct{}

func (e *Extractor) ExtractFromRun(run api.AgentRun) (*api.TestScenario, []byte, error) {
    scenario := &api.TestScenario{
        ID:          "scenario_" + run.RunID,
        Version:     "1.0",
        Description: "TODO: Add description of expected behavior for this scenario.",
        Task:        run.Task,
        Environment: api.EnvironmentSpec{
            FixtureStrategy: api.MatchStrategyHybrid,
            Fixtures:        make([]api.Fixture, 0),
        },
        // CRITICAL: Assertions are intentionally left empty!
        // We never synthesize assertions from run.Trace to avoid enshrining bugs.
        Assertions: []api.Assertion{},
        Reliability: api.ReliabilityConfig{
            Samples:         10,
            MinimumPassRate: 0.95,
            Confidence:      0.95,
        },
        Provenance: api.TestScenarioProvenance{
            Source:      "production_trace",
            SourceRunID: run.RunID,
            ExtractedAt: time.Now().UTC(),
            ReviewedBy:  "", // Must be filled by human reviewer before CI acceptance
        },
    }

    // Extract tool calls as candidate fixtures
    for i, span := range run.Trace {
        if span.Type == api.SpanTypeTool {
            inputMap, _ := span.Attributes["input"].(map[string]any)
            outputBody := span.Attributes["output"]

            hash, _ := jcs.ContentHash(inputMap)

            fx := api.Fixture{
                FixtureID:     fmt.Sprintf("fx_%s_%03d", span.Name, i+1),
                Tool:          span.Name,
                InputHash:     hash,
                MatchStrategy: api.MatchStrategyExactHash,
                RecordedInput: inputMap,
                RecordedResponse: api.RecordedResponse{
                    Status: "success",
                    Body:   outputBody,
                },
                Provenance: api.ProvenanceRecorded,
            }
            scenario.Environment.Fixtures = append(scenario.Environment.Fixtures, fx)
        }
    }

    yamlBytes, err := e.renderYAMLTemplate(scenario)
    return scenario, yamlBytes, err
}
```

### 3.2 Rendered YAML Output
The generated YAML explicitly prompts the engineer to declare correct behavior:
```yaml
id: scenario_run_18291
version: "1.0"
description: "TODO: Add description of expected behavior for this scenario."

task:
  id: refund-001
  input: "Cancel my latest order"

environment:
  fixture_strategy: prefer_exact_then_sequence
  fixtures:
    - fixture_id: fx_get_orders_001
      tool: get_orders
      input_hash: sha256:8f2a...
      recorded_response:
        body:
          - id: 122
            status: DELIVERED
          - id: 123
            status: PROCESSING

# -----------------------------------------------------------------------------
# WARNING: Trace is not the test!
# Assertions are left blank by design. Declare what the agent SHOULD do here,
# not merely what happened in the original captured trace.
# -----------------------------------------------------------------------------
assertions:
  # - id: assert_correct_cancel
  #   type: tool_call
  #   tool: cancel_order
  #   arguments: { order_id: 123 }
  # - id: assert_no_cancel_delivered
  #   type: forbidden_tool_call
  #   tool: cancel_order
  #   arguments: { order_id: 122 }

reliability:
  samples: 10
  minimum_pass_rate: 0.95
  confidence: 0.95

provenance:
  source: production_trace
  source_run_id: run_18291
  extracted_at: 2026-09-06T16:50:00Z
  reviewed_by: "" # Enter your email after authoring assertions
```

---

## 4. Content-Addressed Datasets (`internal/core/dataset/`)

Bundles multiple `TestScenario` files into a content-addressed, versioned test suite via `gust dataset bundle` / `verify`:
1. Calculates content hash of included scenarios using RFC 8785 JCS.
2. Emits `dataset.json` with manifest and content checksums.
3. Refuses silent overwrite when an existing scenario ID hashes differently (use `--force`). Orphan `*.json` files are pruned only with `--force`.
4. `Verify` fails immediately if a scenario file is modified after bundling.

---

## 5. Verification & Test Plan

- **Extraction Invariant Test (`tests/unit/extractor_test.go`):**
  - Provide a buggy `AgentRun` (e.g., agent called `cancel_order(122)` instead of `123`).
  - Run extraction.
  - Assert that `scenario.Assertions` is completely empty.
  - Assert that `cancel_order(122)` is NOT present as an asserted expectation.
- **Fixture Extraction Verification:**
  - Verify all tool spans in the trace are converted to fixtures with valid RFC 8785 hashes and `provenance: "recorded"`.
- **Dataset Immutability Test:**
  - Modify a scenario file within a dataset and verify the dataset checksum verification fails immediately.
