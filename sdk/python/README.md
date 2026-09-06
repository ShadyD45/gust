# gust Python SDK

Capture what your Python agent did as a gust `AgentRun`, then gate it with the `gust` binary. The SDK records and invokes; evaluation, statistics, and verdicts stay in Go.

Standard library only — no dependencies to fight with your agent framework.

## Install

```bash
pip install -e sdk/python
```

## Record a run

```python
from gust_sdk import RunRecorder

rec = RunRecorder(
    agent_name="support-agent",
    agent_version="1.4",
    task_id="refund-001",
    task_input="Cancel my latest order",
)

with rec.tool("get_orders", {"customer_id": 42}) as span:
    span.output = get_orders(customer_id=42)

with rec.tool("cancel_order", {"order_id": 123}) as span:
    span.output = cancel_order(order_id=123)

rec.complete(output="Order 123 cancelled successfully.")
rec.write("out/run.json")
```

```bash
./gust analyze out/run.json --assertions tests/assertions.json
```

The `with` block times the span and records exceptions as error spans before re-raising them, so a failing tool still produces a usable trace.

## API

### `RunRecorder`

| Method | Purpose |
|---|---|
| `tool(name, arguments)` | Record a tool call; `arguments` becomes `attributes.input`, which `tool_call` assertions compare against |
| `llm(name, model=...)` | Record a model call (counts toward `max_steps`) |
| `span(name, span_type=...)` | Any span type: `agent`, `llm`, `tool`, `retrieval`, `memory`, `plan`, `error` |
| `record_tool(name, arguments, output=, error=)` | One-shot form for code that cannot use a `with` block |
| `complete(output=)` | Mark the run completed |
| `fail(error, status=)` | Mark it `failed`, `timeout`, or `cancelled` |
| `set_metadata(key, value)` | Attach build id, prompt version, model name |
| `set_assertions([...])` | Embed assertions in `metadata.assertions` (demos; prefer a separate file) |
| `to_dict()` / `to_json()` / `write(path)` | Serialize |

Invalid input raises `AgentRunError` at capture time — inside your process, where the stack trace is useful — rather than producing an artifact that fails to parse later in CI.

### `FixtureClient`

Routes tool calls through the gust mock proxy during Mode 3 sampling, so the agent is live but its dependencies are not:

```python
from gust_sdk import FixtureClient

fixtures = FixtureClient()   # reads AGENTEVAL_FIXTURE_ENDPOINT

def get_orders(customer_id: int):
    if fixtures.enabled:
        return fixtures.call("get_orders", {"customer_id": customer_id})
    return orders_api.list(customer_id)   # production path
```

`enabled` is `False` when no endpoint is configured, which is what lets the same code run in production and under test. Missing fixtures and injected failures raise `FixtureError`.

### `EvaluatorPlugin`

Write a custom evaluator in Python and expose it over the Tier-2 wire protocol:

```python
from gust_sdk import EvaluatorPlugin, serve

class RequiresCitation(EvaluatorPlugin):
    name = "requires_citation"
    version = "1.0.0"

    def evaluate(self, run, expected, context):
        output = (run.get("outcome") or {}).get("output", "")
        if "[doc:" not in output:
            return {"passed": False, "message": "answer has no document citation"}
        return {"passed": True}

if __name__ == "__main__":
    serve(RequiresCitation())
```

Load it from Go and assert on `{"type": "requires_citation"}`. Full walkthrough: [`docs/extending/wire-plugin-python.md`](../../docs/extending/wire-plugin-python.md).

## Examples

| Example | What it shows |
|---|---|
| [`examples/record_and_analyze.py`](examples/record_and_analyze.py) | Record a two-tool trajectory and gate it with `gust analyze` |
| [`examples/wire_evaluator/plugin.py`](examples/wire_evaluator/plugin.py) | A PII-leak evaluator as a wire plugin |

```bash
python sdk/python/examples/record_and_analyze.py --out out/run.json --gust ./gust
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"manifest"}' | python sdk/python/examples/wire_evaluator/plugin.py
```

## Where to hook into your framework

Wherever tool results come back:

| Framework | Hook |
|---|---|
| LangChain | `BaseCallbackHandler.on_tool_start` / `on_tool_end` / `on_tool_error` |
| OpenAI / Anthropic function calling | Your `for tool_call in response.tool_calls:` dispatch loop |
| LlamaIndex | Callback manager events |
| CrewAI / AutoGen | Tool wrappers or observer hooks |

Packaged adapters for these frameworks are planned; the recorder above works with all of them today.

## Development

```bash
pip install -e "sdk/python[dev]"
python -m pytest sdk/python
```

## Compatibility

| SDK version | gust schema | gust CLI |
|---|---|---|
| 0.5.x | `0.5` | 0.5.x |

The `schema_version` the SDK writes must match the CLI's. A mismatch is rejected at validation rather than silently mapped.
