"""Tier-2 wire plugin scaffolding.

Write a custom evaluator in Python by subclassing :class:`EvaluatorPlugin` and
calling :func:`serve`. The transport (newline-delimited JSON-RPC 2.0 over
stdio), the manifest handshake, and error mapping are handled here.

Stdout carries the protocol, so anything you print corrupts the stream; use
stderr or the ``logging`` module for diagnostics.
"""

from __future__ import annotations

import json
import sys
import time
from typing import Any, Dict, IO, List, Optional

PROTOCOL_VERSION = "1.0"

# JSON-RPC 2.0 reserved codes used by this transport.
METHOD_NOT_FOUND = -32601
INTERNAL_ERROR = -32603


class EvaluatorPlugin:
    """Base class for a Python evaluator exposed to gust.

    Subclasses set :attr:`name` and :attr:`version` and implement
    :meth:`evaluate`. ``name`` is what assertions reference by ``type``.
    """

    name: str = "python_evaluator"
    version: str = "0.1.0"
    description: str = ""
    capabilities: Optional[List[str]] = None

    def manifest(self) -> Dict[str, Any]:
        payload = {
            "protocol_version": PROTOCOL_VERSION,
            "kind": "evaluator",
            "name": self.name,
            "version": self.version,
        }
        if self.description:
            payload["description"] = self.description
        if self.capabilities:
            payload["capabilities"] = list(self.capabilities)
        return payload

    def evaluate(
        self,
        run: Dict[str, Any],
        expected: Optional[Dict[str, Any]],
        context: Dict[str, Any],
    ) -> Dict[str, Any]:
        """Evaluate a run and return a partial result.

        Return at least ``passed``; ``score``, ``message``, and ``evidence``
        are filled in around it. ``expected`` may be ``None`` — treat an
        under-specified assertion as a vacuous pass, never as an error.

        Evidence lands in CI logs and artifacts: record where a violation is,
        not the sensitive value itself.
        """
        raise NotImplementedError

    # -- helpers for subclasses -----------------------------------------

    @staticmethod
    def tool_spans(run: Dict[str, Any], name: Optional[str] = None) -> List[Dict[str, Any]]:
        """Return tool spans, optionally filtered by tool name."""
        spans = [s for s in run.get("trace", []) or [] if s.get("type") == "tool"]
        if name is not None:
            spans = [s for s in spans if s.get("name") == name]
        return spans

    @staticmethod
    def tool_arguments(span: Dict[str, Any]) -> Dict[str, Any]:
        """Return a tool span's recorded arguments."""
        return (span.get("attributes") or {}).get("input") or {}

    def _run_evaluate(self, params: Dict[str, Any]) -> Dict[str, Any]:
        started = time.perf_counter_ns()
        result = self.evaluate(
            params.get("run") or {},
            params.get("expected"),
            params.get("context") or {},
        )

        passed = bool(result.get("passed", False))
        payload: Dict[str, Any] = {
            "evaluator_name": self.name,
            "evaluator_version": self.version,
            "passed": passed,
            "score": float(result.get("score", 1.0 if passed else 0.0)),
            "execution_time_ns": time.perf_counter_ns() - started,
        }
        if result.get("message"):
            payload["message"] = result["message"]
        if result.get("evidence"):
            payload["evidence"] = result["evidence"]
        return payload

    def handle(self, request: Dict[str, Any]) -> Dict[str, Any]:
        """Map one JSON-RPC request to its response."""
        response: Dict[str, Any] = {"jsonrpc": "2.0", "id": request.get("id")}
        method = request.get("method")

        try:
            if method == "manifest":
                response["result"] = self.manifest()
            elif method == "evaluate":
                response["result"] = self._run_evaluate(request.get("params") or {})
            else:
                response["error"] = {
                    "code": METHOD_NOT_FOUND,
                    "message": f"unknown method {method!r}",
                }
        except Exception as exc:  # a plugin bug must not kill the stream
            response["error"] = {"code": INTERNAL_ERROR, "message": str(exc)}
        return response


def serve(
    plugin: EvaluatorPlugin,
    stdin: Optional[IO[str]] = None,
    stdout: Optional[IO[str]] = None,
) -> None:
    """Run the plugin's request loop until stdin closes."""
    source = stdin if stdin is not None else sys.stdin
    sink = stdout if stdout is not None else sys.stdout

    for line in source:
        line = line.strip()
        if not line:
            continue
        try:
            request = json.loads(line)
        except json.JSONDecodeError:
            continue  # gust never sends partial lines; ignore noise

        sink.write(json.dumps(plugin.handle(request)) + "\n")
        sink.flush()  # gust reads line by line and will block without this
