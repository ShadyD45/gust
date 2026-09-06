# Phase 5: Mutation Testing Engine (Replay Mode)

## 1. Objectives & Scope
1. Implement the **Mutation Testing Engine** running exclusively in Replay Mode against synthetic or recorded agents.
2. Implement the **9 MVP mutation classes** representing common real-world agent failures.
3. Track typed mutation outcomes (`Applied`, `Skipped`, `Error`) to prevent skewing metrics when a mutation is inapplicable to a specific trace.
4. Calculate and report the flagship trust metrics: **Detection Rate** ($\ge 90\%$) and **False Positive Rate** ($\le 5\%$) on unmutated `GOOD` traces.

---

## 2. Package Architecture

```text
gust/
├── internal/
│   ├── adapters/
│   │   └── mutators/            # 9 MVP Mutation Classes
│   │       ├── remove_tool.go
│   │       ├── wrong_tool.go
│   │       ├── corrupt_argument.go
│   │       ├── duplicate_call.go
│   │       ├── infinite_loop.go
│   │       ├── skip_recovery.go
│   │       ├── excessive_calls.go
│   │       ├── forbidden_tool.go
│   │       ├── change_output.go
│   │       └── mutators_test.go
│   └── core/
│       └── mutate/              # Mutation Runner & Metric Aggregator
│           ├── runner.go
│           ├── metrics.go
│           └── runner_test.go
```

---

## 3. The 9 MVP Mutation Classes

Each mutator implements `ports.Mutator`:
`Mutate(ctx context.Context, run api.AgentRun) (ports.MutationOutcome, error)`

### 3.1 `remove_required_tool`
- **Action**: Locates a tool span in `run.Trace` that is marked as required or critical and removes it from the trajectory.
- **Skip Condition**: If `run.Trace` contains no tool spans, returns `Status: Skipped, SkipReason: "no tool calls in trace"`.

### 3.2 `wrong_tool`
- **Action**: Replaces the tool name in an existing tool span with a plausible but incorrect alternative (e.g. `cancel_order` -> `modify_order`).

### 3.3 `corrupt_argument`
- **Action**: Randomly modifies a valid parameter key or value (e.g., changes `order_id: 123` to `order_id: -9999` or mutates string payload).

### 3.4 `duplicate_call`
- **Action**: Duplicates an existing tool span and inserts it immediately after the original, with identical arguments.

### 3.5 `infinite_loop`
- **Action**: Injects a repeating sequence of identical tool spans (e.g., calling `get_status` 50 times) to simulate planning loops.

### 3.6 `skip_recovery`
- **Action**: Finds an error span followed by an error-recovery tool span; removes the recovery attempt, causing the trajectory to remain unrecovered.
- **Skip Condition**: Trace has no error spans or retry steps.

### 3.7 `excessive_tool_calls`
- **Action**: Injects a barrage of dummy tool spans until the step count exceeds normal limits (e.g. 20+ extra tool calls).

### 3.8 `introduce_forbidden_tool`
- **Action**: Injects a tool span with a prohibited tool name (e.g., `execute_sql`, `delete_database`, `admin_override`).

### 3.9 `change_final_output`
- **Action**: Mutates `run.Outcome.Output` to a contradicting or nonsensical statement.

---

## 4. Mutation Runner & Metric Calculations (`internal/core/mutate/`)

```go
package mutate

import (
    "context"
    "gust/internal/ports"
    "gust/pkg/api"
)

type MutationReport struct {
    TotalMutantsGenerated int                `json:"total_mutants_generated"`
    MutantsApplied        int                `json:"mutants_applied"`
    MutantsSkipped        int                `json:"mutants_skipped"`
    MutantsDetected       int                `json:"mutants_detected"`
    DetectionRate         float64            `json:"detection_rate"`
    FalsePositiveRate     float64            `json:"false_positive_rate"`
    ClassBreakdown        map[string]ClassStats `json:"class_breakdown"`
}

type ClassStats struct {
    Applied  int     `json:"applied"`
    Detected int     `json:"detected"`
    Rate     float64 `json:"rate"`
}
```

### 4.1 Detection Rate Formula
$$\text{Detection Rate} = \frac{\text{Mutants Detected (Killed)}}{\text{Mutants Successfully Applied}}$$

### 4.2 False Positive Rate Formula
Evaluates original, unmutated `GOOD` golden traces through the evaluator suite:
$$\text{False Positive Rate} = \frac{\text{Unmutated Traces Incorrectly Flagged as Failed}}{\text{Total Unmutated Traces}}$$

---

## 5. Verification & Test Plan

- **Unit Tests for Every Mutator:**
  - Verify that mutated runs contain the expected structural change.
  - Verify that invalid/inapplicable traces return `Status: Skipped` rather than panicking or skewing metrics.
- **Detection Benchmark (`tests/mutation/benchmark_test.go`):**
  - Run the mutation engine against a golden test suite of 100 synthetic agents.
  - Verify that Detection Rate $\ge 90\%$.
  - Verify that False Positive Rate $\le 5\%$.
  - Verify median mutation evaluation latency $\le 5$ ms per mutant.
