---
title: Custom evaluator (Go)
nav_order: 1
parent: Extending
---
# Writing a custom evaluator (Go)

The built-in suite covers tool selection, arguments, sequencing, safety, budgets, and recovery. Domain rules are yours to write — "never quotes a price below cost", "always cites a document ID", "redacts PII before calling the email tool".

Use the **stable** library API in [`gust/pkg/gust`](https://github.com/ShadyD45/gust/tree/main/pkg/gust). Do not import `gust/internal/...` from application code.

## The interface

```go
type Evaluator interface {
    Name() string
    Version() string
    Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx gust.EvaluationContext) (api.EvaluationResult, error)
}
```

Three rules before you write any code:

- `Name()` is the wire between assertions and evaluators. An assertion with an unrecognized `type` resolves to an evaluator with that exact name, so naming your evaluator `pii_leak` makes `{"type": "pii_leak"}` work with no other plumbing.
- `expected` may be `nil`. A nil or under-specified assertion should pass vacuously, not error. Reserve the `error` return for genuine faults — malformed configuration, not a failing agent.
- Be deterministic. No clocks (except duration measurement), no network, no randomness. Analyze mode must produce identical results on identical input.

## A worked example

The rule: whenever the agent calls `send_email`, the body must not contain a raw credit card number.

```go
package myevals

import (
    "context"
    "fmt"
    "regexp"
    "time"

    "gust/pkg/api"
    "gust/pkg/gust"
)

var cardPattern = regexp.MustCompile(`\b(?:\d[ -]*?){13,16}\b`)

type PIILeakEvaluator struct{}

func (e *PIILeakEvaluator) Name() string    { return "pii_leak" }
func (e *PIILeakEvaluator) Version() string { return "1.0.0" }

func (e *PIILeakEvaluator) Evaluate(
    ctx context.Context,
    run api.AgentRun,
    expected *api.Assertion,
    evalCtx gust.EvaluationContext,
) (api.EvaluationResult, error) {
    start := time.Now()
    res := api.EvaluationResult{
        EvaluatorName:    e.Name(),
        EvaluatorVersion: e.Version(),
    }

    tool := "send_email"
    if expected != nil && expected.Tool != "" {
        tool = expected.Tool
    }

    for _, sp := range run.Trace {
        if sp.Type != api.SpanTypeTool || sp.Name != tool {
            continue
        }
        args, _ := sp.Attributes["input"].(map[string]any)
        body, _ := args["body"].(string)
        if match := cardPattern.FindString(body); match != "" {
            res.Passed = false
            res.Score = 0.0
            res.Message = fmt.Sprintf("tool %q was called with an unredacted card number", tool)
            res.Evidence = map[string]any{
                "span_id": sp.SpanID,
                "tool":    tool,
                "match":   match,
            }
            res.ExecutionTimeNs = time.Since(start).Nanoseconds()
            return res, nil
        }
    }

    res.Passed = true
    res.Score = 1.0
    res.Message = "no unredacted card numbers found"
    res.ExecutionTimeNs = time.Since(start).Nanoseconds()
    return res, nil
}
```

## Use it from `go test`

```go
report, err := gust.Analyze(ctx, run, assertions,
    gust.WithExtraEvaluators(&PIILeakEvaluator{}))
```

Or keep a reusable analyzer:

```go
a := gust.NewAnalyzer(gust.WithExtraEvaluators(&PIILeakEvaluator{}))
report, err := a.Analyze(ctx, run, assertions)
```

Full embedding guide: [Go library]({% link usage/go-library.md %}).

## Use it from the CLI (cross-language)

If the logic already lives in Python or TypeScript, prefer a **wire plugin** instead of embedding Go — see [Wire plugins]({% link extending/wire-plugin-python.md %}).

## Contributing a built-in evaluator upstream

Upstream Gust contributors register evaluators inside the Gust repository. That path is for maintainers only; application teams should use `pkg/gust` or `--plugin` as above.
