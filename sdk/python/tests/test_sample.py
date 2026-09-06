import io
import json
import os
import threading
import urllib.request

from gust_sdk import RunRecorder, run_sample, serve_sample


def _handler(request, fixtures):
    rec = RunRecorder(
        agent_name="support-agent",
        agent_version="1.4",
        task_id="refund-001",
        task_input=request.get("input") or "hi",
    )
    rec.complete(output="ok")
    return rec


def test_run_sample_stamps_sample_id():
    stdin = io.StringIO(json.dumps({"input": "Cancel", "sample_id": "sid-1"}))
    stdout = io.StringIO()
    run = run_sample(_handler, stdin=stdin, stdout=stdout)
    assert run["run_id"] == "sid-1"
    assert run["metadata"]["sample_id"] == "sid-1"
    written = json.loads(stdout.getvalue())
    assert written["run_id"] == "sid-1"


def test_serve_sample_invoke(monkeypatch):
    monkeypatch.delenv("AGENTEVAL_SAMPLE_ID", raising=False)
    server = serve_sample(_handler, host="127.0.0.1", port=0)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    host, port = server.server_address[:2]
    try:
        req = urllib.request.Request(
            f"http://{host}:{port}/invoke",
            data=json.dumps({"input": "Cancel", "sample_id": "http-9"}).encode(),
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        with urllib.request.urlopen(req, timeout=5) as resp:
            body = json.loads(resp.read().decode())
        assert body["run_id"] == "http-9"
        assert body["metadata"]["sample_id"] == "http-9"
    finally:
        server.shutdown()


def test_apply_invoke_env_stamps_otel(monkeypatch):
    monkeypatch.delenv("OTEL_EXPORTER_OTLP_ENDPOINT", raising=False)
    monkeypatch.delenv("AGENTEVAL_INGEST_URL", raising=False)
    monkeypatch.delenv("OTEL_RESOURCE_ATTRIBUTES", raising=False)
    from gust_sdk.sample import apply_invoke_env

    sid = apply_invoke_env({"sample_id": "s1", "otel_endpoint": "http://127.0.0.1:4318"})
    assert sid == "s1"
    assert os.environ["OTEL_EXPORTER_OTLP_ENDPOINT"] == "http://127.0.0.1:4318"
    assert os.environ["AGENTEVAL_INGEST_URL"] == "http://127.0.0.1:4318/v1/runs"
    assert "gust.sample_id=s1" in os.environ["OTEL_RESOURCE_ATTRIBUTES"]
    monkeypatch.delenv("AGENTEVAL_INGEST_URL", raising=False)
