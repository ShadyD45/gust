---
title: Custom test runner
nav_order: 3
parent: Extending
---
# Custom test runners

**Application teams should not implement a Go test runner.** To evaluate your agent in Mode 3, use the CLI:

```bash
gust test evals/ --runner exec -- python -m my_agent.gust_eval
gust test evals/ --runner http --endpoint http://localhost:8080
gust test evals/ --runner trigger --trace-fetch-command 'my-cli get {trace_id}' -- -- harness.py
```

See [Test your agent]({% link usage/test-your-agent.md %}) and the [live-agent demo]({% link usage/live-agent-demo.md %}).

## Built-in runners

| Runner | When to use |
|--------|-------------|
| `exec` | Gust starts your process per sample (CI default) |
| `http` | Gust POSTs to a running QA/dev service |
| `trigger` | Your harness starts the sample; Gust fetches the trace by `{trace_id}` |
| `synthetic` / `ollama` | Gust demos and self-checks |

## Go library embedding

For **Analyze** (offline assertions on a captured `AgentRun`), use the stable library API — [Go library]({% link usage/go-library.md %}). That path does not require implementing a runner.

Live N-sample Mode 3 stays on the CLI so Gust can own fixtures, sampling, Wilson scoring, policy, and HTML reports without coupling your module to Gust internals.

## Upstream contributors only

Adding a new built-in runner to the Gust binary is a Gust-repository change for maintainers. Application code must not import `gust/internal/...`.
