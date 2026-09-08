"""One-sample harness so ``gust test --runner http|exec|trigger`` can drive a Python agent.

End users implement a handler that runs the agent once and returns an AgentRun,
or a normal result when an existing tracer already captures the run.
gust owns sampling, optional fixtures, and the verdict.
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
EVALUATION_ID_ENV = "GUST_EVALUATION_ID"
SCENARIO_ID_ENV = "GUST_SCENARIO_ID"
TRACE_ID_ENV = "GUST_TRACE_ID"
TRACEPARENT_ENV = "TRACEPARENT"
BAGGAGE_ENV = "BAGGAGE"
WORLD_CONTROL_ENV = "GUST_WORLD_CONTROL"
RESOURCE_ATTRS_ENV = "OTEL_RESOURCE_ATTRIBUTES"
SAMPLE_ID_ATTR = "gust.sample_id"

SampleHandler = Callable[[Dict[str, Any], FixtureClient], Union[Dict[str, Any], RunRecorder]]


def sample_context() -> Dict[str, Any]:
    """Read the Gust sample context from process environment."""
    return {
        "evaluation_id": os.environ.get(EVALUATION_ID_ENV, ""),
        "scenario_id": os.environ.get(SCENARIO_ID_ENV, ""),
        "sample_id": os.environ.get(SAMPLE_ID_ENV, ""),
        "trace_id": os.environ.get(TRACE_ID_ENV, ""),
        "traceparent": os.environ.get(TRACEPARENT_ENV, ""),
        "baggage": os.environ.get(BAGGAGE_ENV, ""),
        "world_control": os.environ.get(WORLD_CONTROL_ENV, ""),
        "tool_endpoint": os.environ.get(FIXTURE_ENDPOINT_ENV, ""),
        "input": os.environ.get("AGENTEVAL_TASK_INPUT", ""),
    }


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
    """Read the invoke payload gust sends on stdin (exec/trigger runner)."""
    source = stdin if stdin is not None else sys.stdin
    raw = source.read()
    if not raw.strip():
        return sample_context()
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
    for key, env_name in (
        ("evaluation_id", EVALUATION_ID_ENV),
        ("scenario_id", SCENARIO_ID_ENV),
        ("trace_id", TRACE_ID_ENV),
        ("traceparent", TRACEPARENT_ENV),
        ("baggage", BAGGAGE_ENV),
        ("world_control", WORLD_CONTROL_ENV),
    ):
        val = request.get(key)
        if val:
            os.environ[env_name] = str(val)
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


def run_eval(handler: Callable[[Dict[str, Any]], Union[Dict[str, Any], RunRecorder, None]], stdin: Optional[IO[str]] = None, stdout: Optional[IO[str]] = None) -> Dict[str, Any]:
    """Minimal eval entrypoint: one function over scenario input/context.

    Use this when world control is ``existing`` (your own mocks/DI) and you either
    return an AgentRun / RunRecorder or rely on OTel already capturing the run.
    """

    def _wrap(request: Dict[str, Any], fixtures: FixtureClient) -> Union[Dict[str, Any], RunRecorder]:
        result = handler(request)
        if result is None:
            # Placeholder completed run when an external tracer owns observation.
            rec = RunRecorder(
                agent_name=os.environ.get("GUST_AGENT_NAME", "agent"),
                agent_version=os.environ.get("GUST_AGENT_VERSION", "0"),
                task_input=str(request.get("input") or ""),
            )
            rec.complete(output="traced-externally")
            return rec
        return result

    return run_sample(_wrap, stdin=stdin, stdout=stdout)


def execution_receipt(*, status: str = "completed", trace_id: str = "", run_id: str = "", run: Optional[Dict[str, Any]] = None, error: str = "") -> Dict[str, Any]:
    """Build a trigger-runner completion receipt for remote QA / harness flows."""
    out: Dict[str, Any] = {"status": status}
    if trace_id:
        out["trace_id"] = trace_id
    if run_id:
        out["run_id"] = run_id
    if error:
        out["error"] = error
    if run is not None:
        out["run"] = run
    return out


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
            if not request.get("traceparent"):
                request["traceparent"] = self.headers.get("traceparent") or ""
            if not request.get("baggage"):
                request["baggage"] = self.headers.get("baggage") or ""
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
