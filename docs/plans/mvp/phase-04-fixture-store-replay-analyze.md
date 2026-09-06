# Phase 4: Fixture Store, Tool Mock Proxy, Analyze & Replay Modes

## 1. Objectives & Scope
1. Implement a content-addressed, filesystem-backed **Fixture Store**.
2. Support both **Stateless Canonical Hash Matching** and **Stateful Sequential FIFO Matching** to handle repetitive tool calls and polling loops cleanly.
3. Build an ephemeral, local **Tool Mock Proxy Server** (HTTP & JSON-RPC) allowing live agents to execute against controlled fixtures in Test mode.
4. Implement all 5 failure-injection modes: `success`, `timeout`, `malformed`, `slow`, `partial_failure`.
5. Implement **Mode 1 (Analyze)** and **Mode 2 (Replay)** engines with verified 100% determinism.

---

## 2. Package Architecture

```text
gust/
├── internal/
│   ├── adapters/
│   │   └── fixtures/
│   │       ├── store.go         # Content-addressed filesystem store
│   │       ├── memory.go        # Thread-safe in-memory provider
│   │       ├── stateful.go      # Sequential FIFO queue provider
│   │       ├── injector.go      # Failure mode injector
│   │       ├── server.go        # Ephemeral Tool Mock Proxy HTTP/JSON-RPC server
│   │       └── server_test.go
│   └── core/
│       ├── analyze/             # Mode 1 Engine
│       │   ├── engine.go
│       │   └── engine_test.go
│       └── replay/              # Mode 2 Engine
│           ├── engine.go
│           └── engine_test.go
```

---

## 3. Fixture Store & Stateful Resolution

### 3.1 Resolution Strategies
When a tool invocation arrives:
1. **`exact_hash`**:
   $$\text{Hash} = \text{SHA256}(\text{JCS}(\text{ToolArguments}))$$
   Lookup entry matching `(tool_name, Hash)`.
2. **`ordered_sequence`**:
   Maintains a call counter / queue per tool name. The first call gets index 0, second gets index 1, etc. Enables polling simulation (e.g. `PENDING` -> `PENDING` -> `COMPLETED`).
3. **`prefer_exact_then_sequence`**:
   Attempts exact hash lookup first; if no match found, falls back to the sequential queue for that tool.

### 3.2 In-Memory Provider (`internal/adapters/fixtures/memory.go`)
```go
package fixtures

import (
    "context"
    "fmt"
    "sync"
    "gust/internal/ports"
    "gust/pkg/api"
    "gust/pkg/jcs"
)

type MemoryFixtureProvider struct {
    mu            sync.Mutex
    exactFixtures map[string]api.Fixture         // key: tool_name + ":" + input_hash
    seqFixtures   map[string][]api.Fixture       // key: tool_name -> list of fixtures
    seqCounters   map[string]int                 // key: tool_name -> current index
}

func (p *MemoryFixtureProvider) Lookup(ctx context.Context, call ports.ToolCall) (api.RecordedResponse, bool, error) {
    p.mu.Lock()
    defer p.mu.Unlock()

    // 1. Try exact hash match
    hash, err := jcs.ContentHash(call.Arguments)
    if err == nil {
        key := fmt.Sprintf("%s:%s", call.Name, hash)
        if fx, ok := p.exactFixtures[key]; ok {
            return fx.RecordedResponse, true, nil
        }
    }

    // 2. Try sequence match
    if seq, ok := p.seqFixtures[call.Name]; ok {
        idx := p.seqCounters[call.Name]
        if idx < len(seq) {
            p.seqCounters[call.Name]++
            return seq[idx].RecordedResponse, true, nil
        }
    }

    return api.RecordedResponse{}, false, nil
}

func (p *MemoryFixtureProvider) Reset() error {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.seqCounters = make(map[string]int)
    return nil
}
```

---

## 4. Ephemeral Tool Mock Proxy Server (`internal/adapters/fixtures/server.go`)

In Mode 3 (Test), live agents need to call mock tools without modifying their codebase:
1. When a test starts, gust starts an ephemeral HTTP/JSON-RPC server on `127.0.0.1:0` (random available OS port).
2. The endpoint URL is passed to the agent runner via environment variable:
   `gust_FIXTURE_ENDPOINT=http://127.0.0.1:51829/v1/tools`
3. The server handles POST requests:
   - Endpoint `/v1/tools/call` accepting `{"tool": "get_orders", "arguments": {...}}`.
   - Endpoint `/mcp` accepting Model Context Protocol tool invocation requests.
4. The proxy executes failure mode delays or errors before writing the HTTP response:
   - `timeout`: Sleep past request deadline.
   - `slow`: Injects `delay_ms`.
   - `malformed`: Writes invalid JSON or truncated bytes.
   - `partial_failure`: Returns HTTP 500 or HTTP 429.

---

## 5. Mode 1 (Analyze) & Mode 2 (Replay) Engines

### 5.1 Mode 1: Analyze (`internal/core/analyze/engine.go`)
- **Input**: Pre-existing `api.AgentRun`, evaluation suite, policy.
- **Action**: Zero execution, zero mock server. Simply iterates over assertions, evaluates the trace, aggregates results, and produces a structured verdict.
- **Guarantee**: Total determinism, zero network calls.

### 5.2 Mode 2: Replay (`internal/core/replay/engine.go`)
- **Input**: Pre-existing `api.AgentRun` or synthetic agent definition, and a `FixtureProvider`.
- **Action**: Deterministically re-executes the agent control flow. Replaces external tool calls with fixture responses. Feeds recorded or stubbed LLM completions (no live LLM). Emits a new `AgentRun`.
- **Purpose**: Regression testing of evaluators, performance benchmarking, and mutation testing (§Phase 5).

---

## 6. Verification & Test Plan

- **Determinism Test (`tests/replay/determinism_test.go`):**
  - Run the same Replay case 100 times consecutively in parallel.
  - Assert that all 100 generated `AgentRun`s have bit-for-bit identical RFC 8785 content hashes.
- **Sequence Handling Test:**
  - Verify that polling loops correctly progress through consecutive states (`PENDING` -> `COMPLETED`) and don't stall.
- **Failure Injection Test:**
  - Test all 5 failure modes against a mock client and verify correct HTTP status codes, latency delays, and error strings.
- **Network Isolation Test:**
  - Execute Analyze and Replay test suites with host network disconnected (or loopback only) and verify 100% pass rate.
