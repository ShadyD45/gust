"""Python capture SDK for gust.

The SDK records what your agent did as an ``AgentRun`` document. Evaluation
stays in the Go binary — this package never decides whether a run passed.

Typical use::

    from gust_sdk import RunRecorder

    rec = RunRecorder(agent_name="support-agent", agent_version="1.4",
                      task_id="refund-001", task_input="Cancel my latest order")

    with rec.tool("get_orders", {"customer_id": 42}) as span:
        span.output = get_orders(customer_id=42)

    rec.complete(output="Order 123 cancelled.")
    rec.write("run.json")

Then gate it::

    gust analyze run.json --assertions tests/assertions.json
"""

from gust_sdk.fixtures import FixtureClient, FixtureError
from gust_sdk.recorder import (
    SCHEMA_VERSION,
    AgentRunError,
    RunRecorder,
    SpanHandle,
)
from gust_sdk.wire import EvaluatorPlugin, serve

__all__ = [
    "SCHEMA_VERSION",
    "AgentRunError",
    "EvaluatorPlugin",
    "FixtureClient",
    "FixtureError",
    "RunRecorder",
    "SpanHandle",
    "serve",
]

__version__ = "0.5.0"
