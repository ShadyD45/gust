"""One-sample harness so ``gust test --runner http|exec`` can drive a Python agent.

End users implement a handler that runs the agent once and returns an AgentRun.
gust owns sampling, fixtures, and the verdict.
"""

from __future__ import annotations

import json
import os
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any, Callable, Dict, IO, Optional, Union
from urllib.parse import urlparse

from gust_sdk.fixtures import FIXTURE_ENDPOINT_ENV, FixtureClient
from gust_sdk.recorder import INGEST_URL_ENV, OTEL_ENDPOINT_ENV, RunRecorder, post_run

SAMPLE_ID_ENV = "AGENTEVAL_SAMPLE_ID"
RESOURCE_ATTRS_ENV = "OTEL_RESOURCE_ATTRIBUTES"
SAMPLE_ID_ATTR = "gust.sample_id"

SampleHandler = Callable[[Dict[str, Any], FixtureClient], Union[Dict[str, Any], RunRecorder]]


def apply_sample_id(run: Union[Dict[str, Any], RunRecorder], sample_id: str) -> Union[Dict[str, Any], RunRecorder]:
    """Stamp ``sample_id`` onto a recorder or an AgentRun dict."""
    if not sample_id:
        return run
    if isinstance(run, RunRecorder):
        run.run_id = sample_id
        run.set_metadata("sample_id", sample_id)
        return run
    run["run_id"] = sample_id
    metadata = dict(run.get("metadata") or {})
    metadata["sample_id"] = sample_id
    run["metadata"] = metadata
    return run


def read_invoke(stdin: Optional[IO[str]] = None) -> Dict[str, Any]:
    """Read the invoke payload gust sends on stdin (exec runner)."""
    source = stdin if stdin is not None else sys.stdin
    raw = source.read()
    if not raw.strip():
        return {
            "input": os.environ.get("AGENTEVAL_TASK_INPUT", ""),
            "sample_id": os.environ.get(SAMPLE_ID_ENV, ""),
            "tool_endpoint": os.environ.get(FIXTURE_ENDPOINT_ENV, ""),
        }
    return json.loads(raw)


def _as_dict(run: Union[Dict[str, Any], RunRecorder], sample_id: str) -> Dict[str, Any]:
    stamped = apply_sample_id(run, sample_id)
    if isinstance(stamped, RunRecorder):
        return stamped.to_dict()
    return stamped


def apply_invoke_env(request: Dict[str, Any]) -> str:
    """Copy invoke fields onto the process env so OTel SDKs and export() work."""
    sample_id = str(request.get("sample_id") or os.environ.get(SAMPLE_ID_ENV, ""))
    if sample_id:
        os.environ[SAMPLE_ID_ENV] = sample_id
    endpoint = request.get("tool_endpoint") or os.environ.get(FIXTURE_ENDPOINT_ENV) or None
    if endpoint:
        os.environ[FIXTURE_ENDPOINT_ENV] = str(endpoint)
    otel = str(request.get("otel_endpoint") or os.environ.get(OTEL_ENDPOINT_ENV) or "").rstrip("/")
    ingest = str(request.get("ingest_url") or os.environ.get(INGEST_URL_ENV) or "")
    if otel:
        os.environ.setdefault(OTEL_ENDPOINT_ENV, otel)
        os.environ.setdefault("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")
        os.environ.setdefault("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", otel + "/v1/traces")
        if not ingest:
            ingest = otel + "/v1/runs"
    if ingest:
        os.environ.setdefault(INGEST_URL_ENV, ingest)
    if sample_id:
        existing = os.environ.get(RESOURCE_ATTRS_ENV, "")
        token = SAMPLE_ID_ATTR + "=" + sample_id
        if token not in existing.split(","):
            os.environ[RESOURCE_ATTRS_ENV] = (existing + "," + token).strip(",") if existing else token
    return sample_id


def _maybe_export(run: Dict[str, Any]) -> None:
    if os.environ.get(INGEST_URL_ENV):
        post_run(run)


def run_sample(handler: SampleHandler, stdin: Optional[IO[str]] = None, stdout: Optional[IO[str]] = None) -> Dict[str, Any]:
    """Run one sample for ``--runner exec``: stdin JSON in, AgentRun JSON out."""
    request = read_invoke(stdin)
    sample_id = apply_invoke_env(request)
    endpoint = request.get("tool_endpoint") or os.environ.get(FIXTURE_ENDPOINT_ENV) or None
    fixtures = FixtureClient(endpoint=endpoint)
    run = _as_dict(handler(request, fixtures), sample_id)
    _maybe_export(run)
    sink = stdout if stdout is not None else sys.stdout
    sink.write(json.dumps(run) + "\n")
    sink.flush()
    return run


def serve_sample(handler: SampleHandler, host: str = "127.0.0.1", port: int = 8080) -> ThreadingHTTPServer:
    """Serve ``POST /invoke`` for ``--runner http``. Returns the started server.

    Call ``server.serve_forever()`` in your ``__main__`` block.
    """

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, fmt: str, *args: Any) -> None:
            sys.stderr.write("%s - %s\n" % (self.address_string(), fmt % args))

        def do_POST(self) -> None:  # noqa: N802
            path = urlparse(self.path).path
            if path not in ("/invoke", "/"):
                self.send_error(404)
                return
            length = int(self.headers.get("Content-Length", "0"))
            raw = self.rfile.read(length) if length else b"{}"
            try:
                request = json.loads(raw.decode("utf-8") or "{}")
            except json.JSONDecodeError:
                self.send_error(400, "invalid JSON")
                return
            if not request.get("sample_id"):
                request["sample_id"] = self.headers.get("X-Gust-Sample-Id") or os.environ.get(SAMPLE_ID_ENV, "")
            sample_id = apply_invoke_env(request)
            endpoint = request.get("tool_endpoint") or os.environ.get(FIXTURE_ENDPOINT_ENV) or None
            fixtures = FixtureClient(endpoint=endpoint)
            try:
                run = _as_dict(handler(request, fixtures), sample_id)
                _maybe_export(run)
            except Exception as exc:  # surface as a failed run, not a 500, when possible
                self.send_error(500, str(exc))
                return
            body = json.dumps(run).encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

    server = ThreadingHTTPServer((host, port), Handler)
    return server
