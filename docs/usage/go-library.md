---
title: Go library
nav_order: 3.7
parent: Usage
---
# Embed Gust as a Go library

Use the **stable** package [`gust/pkg/gust`](https://github.com/ShadyD45/gust/tree/main/pkg/gust) when you want Mode 1 (Analyze) inside ordinary `go test` or a service — without shelling out to the CLI.

Do **not** import `gust/internal/...` from application code. Internal packages are not semver-stable.

Types (`AgentRun`, `Assertion`, …) live in [`gust/pkg/api`](https://github.com/ShadyD45/gust/tree/main/pkg/api).

## Analyze a captured run

```go
package agent_test

import (
    "context"
    "testing"

    "gust/pkg/api"
    "gust/pkg/gust"
)

func TestCancelBehavior(t *testing.T) {
    run := /* build or load an api.AgentRun */
    assertions := []api.Assertion{
        {ID: "ok", Type: api.AssertTaskSuccess},
        {ID: "cancel", Type: api.AssertToolCall, Tool: "cancel_order",
            Arguments: map[string]any{"order_id": 123}},
    }

    report, err := gust.Analyze(context.Background(), run, assertions,
        gust.WithScenarioID(run.RunID))
    if err != nil {
        t.Fatal(err)
    }
    if !report.Passed {
        t.Fatalf("agent behavior failed: %+v", report.Results)
    }
}
```

`gust.Analyze` uses the same built-in evaluators as the CLI (`gust.BuiltinEvaluators()`).

## Add a custom evaluator

```go
type PIILeakEvaluator struct{}

func (PIILeakEvaluator) Name() string    { return "pii_leak" }
func (PIILeakEvaluator) Version() string { return "1.0.0" }

func (PIILeakEvaluator) Evaluate(
    ctx context.Context,
    run api.AgentRun,
    expected *api.Assertion,
    evalCtx gust.EvaluationContext,
) (api.EvaluationResult, error) {
    // inspect run.Trace; return Passed=false with Evidence on violation
    return api.EvaluationResult{
        EvaluatorName: "pii_leak", EvaluatorVersion: "1.0.0",
        Passed: true, Score: 1,
    }, nil
}

report, err := gust.Analyze(ctx, run, assertions,
    gust.WithExtraEvaluators(PIILeakEvaluator{}))
```

Assertion YAML/JSON with `"type": "pii_leak"` resolves to that evaluator by name.

Reusable `Analyzer` instance:

```go
a := gust.NewAnalyzer(gust.WithExtraEvaluators(PIILeakEvaluator{}))
report, err := a.Analyze(ctx, run, assertions)
```

## Mode 3 (live sampling)

For N-sample live evaluation, prefer the **CLI** (`gust test --runner exec|http|trigger`). That path owns fixtures, sampling, Wilson scoring, policy, and HTML reports without coupling your app to Gust internals.

See [Test your agent]({% link usage/test-your-agent.md %}) and the [live-agent demo]({% link usage/live-agent-demo.md %}).

## Related

- [Custom evaluator (Go)]({% link extending/custom-evaluator-go.md %}) — fuller evaluator guide
- [Wire plugins]({% link extending/wire-plugin-python.md %}) — Python/TS evaluators without embedding Go
- [Modes cookbook]({% link usage/modes-cookbook.md %}) — Analyze / Replay / Test overview
