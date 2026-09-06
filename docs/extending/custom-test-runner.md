---
title: Custom test runner
nav_order: 3
parent: Extending
---
# Writing a custom test runner

A `TestRunner` is what Mode 3 calls *N* times. It drives your actual agent and hands back a trace. The built-ins — `synthetic` (seeded, no dependencies) and `ollama` (local LLM) — are useful for development, but testing *your* agent means teaching gust how to invoke it.

## The interface

```go
type TestRunner interface {
    Name() string
    Run(ctx context.Context, scenario api.TestScenario, fixtureEndpoint string) (api.AgentRun, error)
}
```

One method, called once per sample. The sampler runs these concurrently (`--concurrency`, default 4), so your implementation must be safe for parallel use — no shared mutable state without a lock.

## The fixture endpoint contract

`fixtureEndpoint` is the single most important parameter. gust hands you the base URL of an ephemeral mock tool proxy; your agent's tool calls must go there instead of to real systems. If you ignore it, your "test" will page someone at 3am by issuing a real refund a hundred times.

Fall back to the environment variable when the argument is empty — that is how out-of-process agents receive it:

```go
if fixtureEndpoint == "" {
    fixtureEndpoint = os.Getenv("AGENTEVAL_FIXTURE_ENDPOINT")
}
```

The proxy exposes one route:

```http
POST {fixtureEndpoint}/v1/tools/call
Content-Type: application/json

{ "tool": "get_orders", "arguments": { "customer_id": 42 } }
```

```json
{ "status": "success", "status_code": 200, "body": [{ "id": 123, "status": "PROCESSING" }] }
```

A `404` means no fixture matched the call — usually a hash mismatch on the arguments, which is itself worth failing on. Fixtures can also inject latency, timeouts, malformed bodies, and 500s; see [failure injection]({% link usage/modes-cookbook.md %}#failure-injection).

## Example: HTTP agent runner

For the common case — your agent is a service with an HTTP endpoint:

```go
package testrunner

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "time"

    "gust/pkg/api"
)

type HTTPAgentRunner struct {
    BaseURL string
    Client  *http.Client
}

func NewHTTPAgentRunner(baseURL string) *HTTPAgentRunner {
    return &HTTPAgentRunner{
        BaseURL: baseURL,
        Client:  &http.Client{Timeout: 60 * time.Second},
    }
}

func (r *HTTPAgentRunner) Name() string { return "http_agent" }

func (r *HTTPAgentRunner) Run(ctx context.Context, scenario api.TestScenario, fixtureEndpoint string) (api.AgentRun, error) {
    if fixtureEndpoint == "" {
        fixtureEndpoint = os.Getenv("AGENTEVAL_FIXTURE_ENDPOINT")
    }

    // Your agent must route its tool calls at fixtureEndpoint for this sample.
    body, err := json.Marshal(map[string]any{
        "input":            scenario.Task.Input,
        "context":          scenario.Task.Context,
        "tool_endpoint":    fixtureEndpoint,
    })
    if err != nil {
        return api.AgentRun{}, err
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.BaseURL+"/invoke", bytes.NewReader(body))
    if err != nil {
        return api.AgentRun{}, err
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := r.Client.Do(req)
    if err != nil {
        return api.AgentRun{}, fmt.Errorf("invoke agent: %w", err)
    }
    defer resp.Body.Close()

    // Your service returns the trace it recorded for this invocation.
    var run api.AgentRun
    if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
        return api.AgentRun{}, fmt.Errorf("decode agent run: %w", err)
    }
    if err := run.Validate(); err != nil {
        return api.AgentRun{}, fmt.Errorf("agent returned an invalid run: %w", err)
    }
    return run, nil
}
```

The recorder inside your service is the same one from [Integrate your app]({% link usage/integrate-your-app.md %}) — a runner is just the piece that triggers it *N* times against controlled fixtures.

## Building the run yourself

If your agent is a Go library rather than a service, construct the trace as you drive it:

```go
func (r *InProcessRunner) Run(ctx context.Context, scenario api.TestScenario, fixtureEndpoint string) (api.AgentRun, error) {
    agent := myagent.New(myagent.WithToolEndpoint(fixtureEndpoint))

    var spans []api.Span
    agent.OnToolCall(func(name string, args map[string]any, out any, start, end time.Time) {
        spans = append(spans, api.Span{
            SpanID:     fmt.Sprintf("s%d", len(spans)+1),
            Name:       name,
            Type:       api.SpanTypeTool,
            StartTime:  start,
            EndTime:    end,
            Attributes: map[string]any{"input": args, "output": out},
            Status:     api.SpanStatus{Code: "ok"},
        })
    })

    output, err := agent.Run(ctx, scenario.Task.Input)
    outcome := api.RunOutcome{Status: "completed", Output: output}
    if err != nil {
        outcome = api.RunOutcome{Status: "failed", Error: err.Error()}
    }

    return api.AgentRun{
        SchemaVersion: api.SchemaVersion,
        RunID:         fmt.Sprintf("%s-%d", scenario.ID, time.Now().UnixNano()),
        Agent:         api.AgentInfo{Name: "my-agent", Version: myagent.Version},
        Task:          scenario.Task,
        Trace:         spans,
        Outcome:       outcome,
    }, nil
}
```

Two things trip people up here:

- **`RunID` must be unique per sample.** Reusing one makes 200 samples indistinguishable in the evidence output.
- **A failed agent run is not a runner error.** Return `outcome.status: "failed"` and a `nil` error. Reserve the error return for infrastructure problems — if you return an error, the whole sampling run aborts rather than counting one failure.

## Registering and using it

```go
func init() {
    _ = registry.DefaultRegistry.RegisterTestRunner(NewHTTPAgentRunner("http://localhost:8080"))
}
```

```bash
./gust test tests/cancel_order.yaml --runner http_agent --samples 100
```

Runner resolution checks the built-in names `synthetic` and `ollama` first, then falls back to the registry, so any registered name works as a `--runner` value.

## Custom fixture providers

To resolve fixtures from somewhere other than memory — a recorded-traffic store, a database, a VCR-style cassette library — implement `ports.FixtureProvider`:

```go
type FixtureProvider interface {
    Lookup(ctx context.Context, call ports.ToolCall) (api.RecordedResponse, bool, error)
    Record(ctx context.Context, call ports.ToolCall, resp api.RecordedResponse) error
    Reset() error
}
```

Semantics to preserve:

- `Lookup` returns `(response, found, error)`. A miss is `found == false` with a `nil` error — not an error.
- Matching should be content-addressed: hash canonical arguments with [`pkg/jcs`](https://github.com/ShadyD45/gust/tree/main/pkg/jcs) so key ordering and whitespace never change the result.
- `Reset` clears sequence counters so stateful ordered fixtures restart cleanly between samples.

Wrap it in the mock proxy to serve it over HTTP:

```go
provider := myfixtures.NewPostgresProvider(db)
proxy, err := fixtures.NewMockToolProxyServer(provider)
if err != nil {
    return err
}
endpoint := proxy.Start()
defer proxy.Close()
```

## Checklist

- [ ] Safe for concurrent calls
- [ ] Routes tool calls through `fixtureEndpoint` / `AGENTEVAL_FIXTURE_ENDPOINT`
- [ ] Unique `RunID` per sample
- [ ] Agent failures reported as `outcome.status`, not as a returned error
- [ ] `run.Validate()` passes before returning
- [ ] Registered with `registry.DefaultRegistry.RegisterTestRunner`

