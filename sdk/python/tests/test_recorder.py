import json

import pytest

from gust_sdk import AgentRunError, RunRecorder


def make_recorder(**kwargs):
    defaults = dict(
        agent_name="support-agent",
        agent_version="1.4",
        task_id="refund-001",
        task_input="Cancel my latest order",
    )
    defaults.update(kwargs)
    return RunRecorder(**defaults)


def test_records_required_agent_run_fields():
    rec = make_recorder(run_id="run-1")
    rec.complete(output="done")
    run = rec.to_dict()

    assert run["schema_version"] == "0.5"
    assert run["run_id"] == "run-1"
    assert run["agent"] == {"name": "support-agent", "version": "1.4"}
    assert run["task"] == {"id": "refund-001", "input": "Cancel my latest order"}
    assert run["outcome"] == {"status": "completed", "output": "done"}


def test_tool_arguments_land_in_attributes_input():
    rec = make_recorder()
    with rec.tool("cancel_order", {"order_id": 123}) as span:
        span.output = {"ok": True}
    rec.complete(output="cancelled")

    tool_span = rec.to_dict()["trace"][0]
    assert tool_span["type"] == "tool"
    assert tool_span["name"] == "cancel_order"
    assert tool_span["attributes"]["input"] == {"order_id": 123}
    assert tool_span["attributes"]["output"] == {"ok": True}
    assert tool_span["status"] == {"code": "ok"}


def test_span_ids_are_unique_and_ordered():
    rec = make_recorder()
    for i in range(5):
        rec.record_tool(f"tool_{i}", {"i": i}, output=i)
    rec.complete()

    ids = [span["span_id"] for span in rec.to_dict()["trace"]]
    assert ids == ["s1", "s2", "s3", "s4", "s5"]


def test_tool_exception_is_recorded_and_reraised():
    rec = make_recorder()

    with pytest.raises(ValueError):
        with rec.tool("cancel_order", {"order_id": 123}):
            raise ValueError("upstream 500")

    rec.fail("upstream 500")
    span = rec.to_dict()["trace"][0]
    assert span["status"]["code"] == "error"
    assert "upstream 500" in span["status"]["message"]
    assert rec.to_dict()["outcome"]["status"] == "failed"


def test_record_tool_with_error():
    rec = make_recorder()
    rec.record_tool("cancel_order", {"order_id": 123}, error="boom")
    rec.complete()

    assert rec.to_dict()["trace"][0]["status"] == {"code": "error", "message": "boom"}


def test_llm_and_custom_spans():
    rec = make_recorder()
    rec.llm("chat", model="llama3.1:8b")
    rec.span("plan_steps", span_type="plan")
    rec.complete()

    types = [span["type"] for span in rec.to_dict()["trace"]]
    assert types == ["llm", "plan"]
    assert rec.to_dict()["trace"][0]["attributes"]["model"] == "llama3.1:8b"


def test_missing_outcome_is_an_error():
    rec = make_recorder()
    with pytest.raises(AgentRunError):
        rec.to_dict()


def test_invalid_inputs_fail_at_capture_time():
    with pytest.raises(AgentRunError):
        RunRecorder(agent_name="", agent_version="1.0", task_input="x")
    with pytest.raises(AgentRunError):
        RunRecorder(agent_name="a", agent_version="1.0", task_input="")

    rec = make_recorder()
    with pytest.raises(AgentRunError):
        rec.span("weird", span_type="not_a_span_type")
    with pytest.raises(AgentRunError):
        rec.fail("boom", status="exploded")


def test_metadata_and_assertions():
    rec = make_recorder()
    rec.set_metadata("prompt_version", "v7")
    rec.set_assertions([{"id": "a1", "type": "task_success"}])
    rec.complete()

    metadata = rec.to_dict()["metadata"]
    assert metadata["prompt_version"] == "v7"
    assert metadata["assertions"][0]["type"] == "task_success"


def test_write_creates_parent_directories(tmp_path):
    rec = make_recorder()
    rec.complete(output="done")
    path = rec.write(str(tmp_path / "nested" / "run.json"))

    with open(path, encoding="utf-8") as handle:
        written = json.load(handle)
    assert written["run_id"] == rec.run_id


def test_timestamps_are_utc_zulu():
    rec = make_recorder()
    rec.record_tool("get_orders", {"customer_id": 42}, output=[])
    rec.complete()

    span = rec.to_dict()["trace"][0]
    assert span["start_time"].endswith("Z")
    assert span["end_time"].endswith("Z")


def test_export_posts_agent_run(monkeypatch):
    from http.server import BaseHTTPRequestHandler, HTTPServer
    import threading

    received = {}

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            return

        def do_POST(self):
            length = int(self.headers.get("Content-Length", "0"))
            received["path"] = self.path
            received["body"] = json.loads(self.rfile.read(length))
            self.send_response(202)
            self.end_headers()
            self.wfile.write(b'{"ok":true}')

    server = HTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        rec = make_recorder(run_id="export-1")
        rec.complete(output="done")
        host, port = server.server_address[:2]
        rec.export(url=f"http://{host}:{port}")
        assert received["path"] == "/v1/runs"
        assert received["body"]["run_id"] == "export-1"
    finally:
        server.shutdown()


def test_resolve_ingest_url_from_otel(monkeypatch):
    from gust_sdk.recorder import resolve_ingest_url

    monkeypatch.delenv("AGENTEVAL_INGEST_URL", raising=False)
    monkeypatch.setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:4318")
    assert resolve_ingest_url() == "http://127.0.0.1:4318/v1/runs"
