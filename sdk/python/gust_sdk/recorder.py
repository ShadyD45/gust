"""Record agent trajectories as gust ``AgentRun`` documents."""

from __future__ import annotations

import json
import os
import uuid
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

SCHEMA_VERSION = "0.5"

_SPAN_TYPES = {"agent", "llm", "tool", "retrieval", "memory", "plan", "error"}
_OUTCOME_STATUSES = {"completed", "failed", "timeout", "cancelled"}


class AgentRunError(ValueError):
    """Raised when a recorded run would be rejected by gust.

    Failing here — inside your own process, at capture time — is far cheaper
    than failing later in CI with an unparseable artifact.
    """


def _now() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="microseconds").replace("+00:00", "Z")


class SpanHandle:
    """A span being recorded.

    Set :attr:`output` to attach the tool result. Exceptions raised inside the
    ``with`` block are recorded as an error span and then re-raised — a failing
    tool is data about the run, not a reason to lose the trace.
    """

    def __init__(self, span: Dict[str, Any]) -> None:
        self._span = span
        self.output: Any = None

    @property
    def span_id(self) -> str:
        return self._span["span_id"]

    def set_attribute(self, key: str, value: Any) -> None:
        self._span.setdefault("attributes", {})[key] = value

    def _finish(self, error: Optional[BaseException]) -> None:
        self._span["end_time"] = _now()
        if self.output is not None:
            self._span.setdefault("attributes", {})["output"] = self.output
        if error is not None:
            self._span["status"] = {"code": "error", "message": str(error)}

    def __enter__(self) -> "SpanHandle":
        return self

    def __exit__(self, exc_type, exc, tb) -> bool:
        self._finish(exc)
        return False  # never swallow the caller's exception


class RunRecorder:
    """Builds an ``AgentRun`` from an agent's execution.

    Instances are not thread-safe. Use one recorder per agent invocation,
    which is also what Mode 3 sampling requires (one run per sample).
    """

    def __init__(
        self,
        agent_name: str,
        agent_version: str,
        task_input: str,
        task_id: Optional[str] = None,
        run_id: Optional[str] = None,
        git_commit: Optional[str] = None,
        task_context: Optional[Dict[str, Any]] = None,
    ) -> None:
        if not agent_name or not agent_version:
            raise AgentRunError("agent_name and agent_version are required")
        if not task_input:
            raise AgentRunError("task_input is required")

        self.run_id = run_id or str(uuid.uuid4())
        self._agent = {"name": agent_name, "version": agent_version}
        if git_commit:
            self._agent["git_commit"] = git_commit

        self._task: Dict[str, Any] = {"id": task_id or self.run_id, "input": task_input}
        if task_context:
            self._task["context"] = task_context

        self._spans: List[Dict[str, Any]] = []
        self._outcome: Optional[Dict[str, Any]] = None
        self._metadata: Dict[str, Any] = {}

    # -- span recording -------------------------------------------------

    def span(
        self,
        name: str,
        span_type: str = "agent",
        attributes: Optional[Dict[str, Any]] = None,
        parent_span_id: Optional[str] = None,
    ) -> SpanHandle:
        """Start a span of any type. Prefer :meth:`tool` and :meth:`llm`."""
        if span_type not in _SPAN_TYPES:
            raise AgentRunError(
                f"unknown span type {span_type!r}; expected one of {sorted(_SPAN_TYPES)}"
            )
        if not name:
            raise AgentRunError("span name is required")

        span: Dict[str, Any] = {
            "span_id": f"s{len(self._spans) + 1}",
            "name": name,
            "type": span_type,
            "start_time": _now(),
            "end_time": _now(),
            "status": {"code": "ok"},
        }
        if parent_span_id:
            span["parent_span_id"] = parent_span_id
        if attributes:
            span["attributes"] = dict(attributes)

        self._spans.append(span)
        return SpanHandle(span)

    def tool(
        self,
        name: str,
        arguments: Optional[Dict[str, Any]] = None,
        parent_span_id: Optional[str] = None,
    ) -> SpanHandle:
        """Record a tool call.

        ``arguments`` lands in ``attributes.input``, which is exactly what
        ``tool_call`` assertions compare against.
        """
        return self.span(
            name,
            span_type="tool",
            attributes={"input": dict(arguments or {})},
            parent_span_id=parent_span_id,
        )

    def llm(
        self,
        name: str,
        model: Optional[str] = None,
        parent_span_id: Optional[str] = None,
        **attributes: Any,
    ) -> SpanHandle:
        """Record a model call. Counts toward ``max_steps`` budgets."""
        attrs: Dict[str, Any] = dict(attributes)
        if model:
            attrs["model"] = model
        return self.span(name, span_type="llm", attributes=attrs, parent_span_id=parent_span_id)

    def record_tool(
        self,
        name: str,
        arguments: Optional[Dict[str, Any]] = None,
        output: Any = None,
        error: Optional[str] = None,
    ) -> str:
        """Record a completed tool call in one call, for non-``with`` code paths."""
        handle = self.tool(name, arguments)
        handle.output = output
        handle._finish(RuntimeError(error) if error else None)
        return handle.span_id

    # -- outcome and output ---------------------------------------------

    def complete(self, output: str = "", duration_ns: Optional[int] = None) -> None:
        """Mark the run as successfully completed."""
        self._outcome = {"status": "completed", "output": output}
        if duration_ns is not None:
            self._outcome["duration_ns"] = duration_ns

    def fail(self, error: str, status: str = "failed", output: str = "") -> None:
        """Mark the run as failed, timed out, or cancelled.

        A failed agent run is normal test data. Record it and let the
        evaluators decide, rather than discarding the trace.
        """
        if status not in _OUTCOME_STATUSES:
            raise AgentRunError(
                f"unknown outcome status {status!r}; expected one of {sorted(_OUTCOME_STATUSES)}"
            )
        self._outcome = {"status": status, "error": error, "output": output}

    def set_metadata(self, key: str, value: Any) -> None:
        """Attach free-form metadata (build id, prompt version, model name)."""
        self._metadata[key] = value

    def set_assertions(self, assertions: List[Dict[str, Any]]) -> None:
        """Embed assertions in ``metadata.assertions``.

        Convenient for demos and single-file examples. For real projects keep
        assertions in their own file and pass ``--assertions`` instead, so that
        production capture code carries no test logic.
        """
        self._metadata["assertions"] = assertions

    # -- output ----------------------------------------------------------

    def to_dict(self) -> Dict[str, Any]:
        """Return the ``AgentRun`` document, validating required fields."""
        if self._outcome is None:
            raise AgentRunError(
                "run has no outcome; call complete() or fail() before serializing"
            )

        run: Dict[str, Any] = {
            "schema_version": SCHEMA_VERSION,
            "run_id": self.run_id,
            "agent": self._agent,
            "task": self._task,
            "trace": self._spans,
            "outcome": self._outcome,
        }
        if self._metadata:
            run["metadata"] = self._metadata
        return run

    def to_json(self, indent: int = 2) -> str:
        return json.dumps(self.to_dict(), indent=indent, sort_keys=False, default=str)

    def write(self, path: str) -> str:
        """Write the run to ``path``, creating parent directories as needed."""
        directory = os.path.dirname(os.path.abspath(path))
        os.makedirs(directory, exist_ok=True)
        with open(path, "w", encoding="utf-8") as handle:
            handle.write(self.to_json())
            handle.write("\n")
        return path
