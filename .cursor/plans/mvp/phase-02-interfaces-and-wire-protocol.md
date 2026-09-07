# Phase 2: Extensible Interfaces & Cross-Language Wire Protocol

## 1. Objectives & Scope
1. Define clean, decoupled Go interfaces (Tier 1) for the five core extension points: `Evaluator`, `Mutator`, `FixtureProvider`, `TestRunner`, and `PolicyEngine`.
2. Implement a thread-safe **Plugin Registry** allowing in-process registration and dynamic lookup.
3. Design and implement the **Tier 2 Wire Protocol** using JSON-RPC 2.0 over standard I/O (`stdio`), enabling plugins written in Python, TypeScript, Rust, or any language.
4. Build a robust **Subprocess Supervisor** with execution timeouts, resource limits, and sandboxing for untrusted plugins.

---

## 2. Package Architecture

```text
gust/
├── internal/
│   ├── ports/                   # Tier 1 Interface contracts
│   │   ├── evaluator.go
│   │   ├── mutator.go
│   │   ├── fixture.go
│   │   ├── testrunner.go
│   │   └── policy.go
│   ├── registry/                # Pluggable Component Registry
│   │   ├── registry.go          # Thread-safe component maps
│   │   └── registry_test.go
│   └── adapters/
│       └── wire/                # Tier 2 Cross-Language Protocol Host
│           ├── client.go        # JSON-RPC 2.0 client over stdio
│           ├── supervisor.go    # Process lifecycle, timeouts, health checks
│           ├── sandbox.go       # Environment scrubbing and isolation
│           └── wire_test.go
```

---

## 3. Tier 1 Native Go Interfaces (`internal/ports/`)

All interfaces follow Go best practices: accept `context.Context`, pass data by value or immutable pointer, return typed errors, and avoid side effects.

### 3.1 Evaluator Interface (`internal/ports/evaluator.go`)
```go
package ports

import (
    "context"
    "gust/pkg/api"
)

type EvaluationContext struct {
    ScenarioID  string
    Environment map[string]any
    Config      map[string]any
}

type EvaluationResult struct {
    EvaluatorName    string            `json:"evaluator_name"`
    EvaluatorVersion string            `json:"evaluator_version"`
    Passed           bool              `json:"passed"`
    Score            float64           `json:"score"` // 0.0 to 1.0
    Message          string            `json:"message"`
    Evidence         api.Evidence      `json:"evidence"`
    ExecutionTimeNs  int64             `json:"execution_time_ns"`
}

type Evaluator interface {
    Name() string
    Version() string
    Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx EvaluationContext) (EvaluationResult, error)
}
```

### 3.2 Mutator Interface (`internal/ports/mutator.go`)
```go
package ports

import (
    "context"
    "gust/pkg/api"
)

type MutationClass string

type MutationOutcome struct {
    Status      api.MutationStatus `json:"status"` // "applied", "skipped", "error"
    Class       MutationClass      `json:"class"`
    OriginalRun api.AgentRun       `json:"original_run"`
    MutatedRun  api.AgentRun       `json:"mutated_run,omitempty"`
    SkipReason  string             `json:"skip_reason,omitempty"`
    Description string             `json:"description"`
}

type Mutator interface {
    Name() string
    Class() MutationClass
    Mutate(ctx context.Context, run api.AgentRun) (MutationOutcome, error)
}
```

### 3.3 FixtureProvider & Mock Proxy (`internal/ports/fixture.go`)
```go
package ports

import (
    "context"
    "gust/pkg/api"
)

type ToolCall struct {
    Name      string         `json:"name"`
    Arguments map[string]any `json:"arguments"`
    CallID    string         `json:"call_id,omitempty"`
}

type FixtureProvider interface {
    Lookup(ctx context.Context, call ToolCall) (api.RecordedResponse, bool, error)
    Record(ctx context.Context, call ToolCall, resp api.RecordedResponse) error
    Reset() error // Resets sequence order counters for stateful mocks
}
```

### 3.4 TestRunner Interface (`internal/ports/testrunner.go`)
```go
package ports

import (
    "context"
    "gust/pkg/api"
)

type TestRunner interface {
    Name() string
    // Run executes a live agent against a controlled environment.
    // fixtureEndpoint specifies the local URL/socket of the Mock Tool Proxy.
    Run(ctx context.Context, scenario api.TestScenario, fixtureEndpoint string) (api.AgentRun, error)
}
```

---

## 4. Pluggable Component Registry (`internal/registry/`)

A centralized, thread-safe registry allows zero-recompile extensions:

```go
package registry

import (
    "fmt"
    "sync"
    "gust/internal/ports"
)

type Registry struct {
    mu          sync.RWMutex
    evaluators  map[string]ports.Evaluator
    mutators    map[string]ports.Mutator
    testRunners map[string]ports.TestRunner
}

func New() *Registry {
    return &Registry{
        evaluators:  make(map[string]ports.Evaluator),
        mutators:    make(map[string]ports.Mutator),
        testRunners: make(map[string]ports.TestRunner),
    }
}

func (r *Registry) RegisterEvaluator(e ports.Evaluator) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    if _, exists := r.evaluators[e.Name()]; exists {
        return fmt.Errorf("evaluator %q already registered", e.Name())
    }
    r.evaluators[e.Name()] = e
    return nil
}

func (r *Registry) GetEvaluator(name string) (ports.Evaluator, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    e, ok := r.evaluators[name]
    return e, ok
}
```

---

## 5. Tier 2 Cross-Language Wire Protocol (`internal/adapters/wire/`)

### 5.1 Protocol Specification (JSON-RPC 2.0 over `stdio`)
External plugins run as companion child processes. Communication occurs via line-delimited JSON-RPC 2.0 over stdin/stdout.

1. **Handshake / Metadata Query:**
   - Client sends: `{"jsonrpc": "2.0", "id": 1, "method": "manifest", "params": {}}`
   - Plugin replies:
     ```json
     {
       "jsonrpc": "2.0",
       "id": 1,
       "result": {
         "protocol_version": "1.0",
         "kind": "evaluator",
         "name": "python_sql_safety",
         "version": "0.1.0",
         "description": "Checks SQL AST for unsafe mutations"
       }
     }
     ```

2. **Execution (`evaluate`, `mutate`, `run`):**
   - Client sends: `{"jsonrpc": "2.0", "id": 2, "method": "evaluate", "params": {"run": {...}, "assertion": {...}}}`
   - Plugin replies with result or standard JSON-RPC error.

### 5.2 Subprocess Supervisor & Sandboxing
To ensure security and stability:
1. **Isolation:** Child processes are spawned in an isolated temporary working directory with environment scrubbed (only explicit `PATH` and non-sensitive variables passed).
2. **Network Policy (planned, Phase 8.5):** Untrusted Evaluators and Mutators are intended to run with network isolation (`unshare` on Linux or Windows Job Objects). **Not implemented yet** — today plugins get a scrubbed environment, handshake/call timeouts, and a line-size bound only. `TestRunner` plugins are expected to need outbound network as configured.
3. **Deadlines & Cancellation:** Every request passes a `context.Context` with timeout. If the plugin fails to respond within the deadline, `SIGKILL` / `TerminateProcess` is sent and an `ErrPluginTimeout` is returned.

---

## 6. Verification & Test Plan

- **Contract Tests (`tests/contract/wire_test.go`):**
  - Implement a dummy Python script that speaks JSON-RPC 2.0 over stdio.
  - Test handshake, successful evaluation, error handling, and timeout behavior.
- **Concurrency & Thread-Safety Tests:**
  - Execute 100 concurrent requests across registered Go and Wire-protocol evaluators to ensure zero data races (tested with `go test -race`).
- **Failure Injection Tests:**
  - Simulate a crashing plugin (exits unexpectedly) and verify the supervisor recovers cleanly without hanging or leaking zombie processes.
