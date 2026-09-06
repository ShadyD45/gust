# Phase 3: Deterministic Evaluator Suite & Structured Evidence

## 1. Objectives & Scope
1. Implement the **10 MVP deterministic evaluators** in native Go under `internal/adapters/evaluators/`.
2. Generate rich, human-readable and machine-actionable **Structured Evidence** for every failure.
3. Optimize evaluation routines for extreme performance (target: $\ge 1,000$ evaluations/second per core).
4. Guarantee 100% offline, zero-network, zero-cost operation.

---

## 2. Package Architecture

```text
gust/
├── internal/
│   ├── adapters/
│   │   └── evaluators/
│   │       ├── task_success.go
│   │       ├── tool_selection.go
│   │       ├── tool_arguments.go
│   │       ├── tool_sequence.go
│   │       ├── forbidden_tool.go
│   │       ├── required_tool.go
│   │       ├── max_steps.go
│   │       ├── max_latency.go
│   │       ├── error_recovery.go
│   │       ├── schema_validation.go
│   │       └── evaluators_test.go
│   └── domain/
│       └── evidence/            # Structured Evidence builder and diff generators
│           ├── evidence.go
│           ├── diff.go
│           └── diff_test.go
```

---

## 3. The 10 Built-in Evaluator Implementations

Every evaluator implements the `ports.Evaluator` interface:
`Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx ports.EvaluationContext) (ports.EvaluationResult, error)`

### 3.1 `TaskSuccess`
- **Logic**: Inspects `run.Outcome.Status`. If expected output is declared in assertion parameters, compares string equality, substring inclusion, or regex pattern.
- **Evidence**:
  ```json
  {
    "actual_status": "failed",
    "expected_status": "completed",
    "error_message": "context deadline exceeded"
  }
  ```

### 3.2 `ToolSelection`
- **Logic**: Extracts all `tool` spans from `run.Trace`. Verifies that only allowed/expected tools were called.
- **Evidence**: Lists unexpected tool names or missing expected tool names.

### 3.3 `ToolArguments`
- **Logic**: Matches tool call input arguments against expected JSON values.
  - Exact match (JCS comparison).
  - Subset match (asserted keys must match values, other keys ignored).
  - JSON Schema match on arguments.
- **Evidence**: Detailed JSON diff showing exact paths of mismatched keys or mismatched values.

### 3.4 `ToolSequence`
- **Logic**: Verifies that tools are invoked in the exact declared order or satisfies a directed acyclic graph (DAG) dependency sequence.
- **Evidence**: Out-of-order span ID, expected prior tool vs actual called tool.

### 3.5 `ForbiddenTool`
- **Logic**: Scans trace for any tool invocation matching forbidden tool names or matching forbidden arguments.
- **Evidence**: Cites offending `span_id`, tool name, invocation timestamp, and argument payload.

### 3.6 `RequiredTool`
- **Logic**: Asserts that one or more specific tools were called at least once during execution.
- **Evidence**: List of required tools that never appeared in the trajectory.

### 3.7 `MaxSteps`
- **Logic**: Counts total execution steps (or specifically `tool` + `llm` spans). Fails if step count exceeds `assertion.Limit`.
- **Evidence**: Actual steps taken ($N$) vs limit ($M$), along with step breakdown by span type.

### 3.8 `MaxLatency`
- **Logic**: Calculates total trajectory duration or specific span durations (`run.Outcome.Duration` or `span.EndTime - span.StartTime`).
- **Evidence**: Observed duration in milliseconds vs threshold in milliseconds.

### 3.9 `ErrorRecovery`
- **Logic**: Detects when a tool span returned an error or status code $\ge 400$. Checks whether the agent continued execution, tried an alternative action, or successfully recovered, rather than abruptly halting or entering an infinite retry loop.
- **Evidence**: Cites the initial failure span and the subsequent agent trajectory actions.

### 3.10 `SchemaValidation`
- **Logic**: Validates tool arguments or final output against a specified JSON Schema.
- **Evidence**: List of JSON Schema validation errors, offending paths, and schema constraints violated.

---

## 4. Structured Evidence Generation (`internal/domain/evidence/`)

To fulfill the core principle **"Evidence Over Scores"**, `EvaluationResult.Evidence` contains structured details:

```go
package evidence

type StructuredEvidence struct {
    FailedSpanID string         `json:"failed_span_id,omitempty"`
    ToolName     string         `json:"tool_name,omitempty"`
    Expected     any            `json:"expected,omitempty"`
    Actual       any            `json:"actual,omitempty"`
    Diff         string         `json:"diff,omitempty"`
    Violations   []Violation    `json:"violations,omitempty"`
}

type Violation struct {
    Path    string `json:"path"`
    Message string `json:"message"`
}
```

---

## 5. Performance Optimization & Benchmark Plan

1. **Zero-Allocation Traversals:** Iterate over `run.Trace` using slices without creating intermediate allocations or strings where possible.
2. **Pre-Compiled Regular Expressions:** Cache regex patterns and compiled JSON schemas in thread-safe LRU caches.
3. **Benchmark Target:**
   - Execute benchmark suite: `go test -bench=BenchmarkEvaluators -benchmem ./...`
   - Target: $\ge 1,000$ evaluations per second per core.
   - Allocation budget: $< 5$ KB per evaluation run.

---

## 6. Verification & Test Plan

- **Unit Tests:** For each of the 10 evaluators, write table-driven test cases covering:
  - Clean pass scenarios.
  - Obvious failures.
  - Boundary conditions (empty trace, single span, 100+ spans).
  - Malformed or missing arguments in spans.
- **Golden Evidence Verification:** Confirm that generated diffs and evidence match golden snapshots.
