import json
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer

import pytest

from gust_sdk import FixtureClient, FixtureError
from gust_sdk.fixtures import FIXTURE_ENDPOINT_ENV


class _Handler(BaseHTTPRequestHandler):
    """Stands in for the Go mock tool proxy, same routes and payloads."""

    def do_POST(self):  # noqa: N802 - http.server API
        length = int(self.headers.get("Content-Length", 0))
        request = json.loads(self.rfile.read(length))

        if request["tool"] == "get_orders":
            body = {"status": "success", "status_code": 200, "body": [{"id": 123}]}
            code = 200
        elif request["tool"] == "flaky_tool":
            body = {"status": "error", "error": "injected simulated server error"}
            code = 500
        else:
            body = {"status": "error", "error": f'no fixture found for tool "{request["tool"]}"'}
            code = 404

        payload = json.dumps(body).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def do_GET(self):  # noqa: N802 - http.server API
        self.send_response(200 if self.path == "/health" else 404)
        self.send_header("Content-Length", "2")
        self.end_headers()
        self.wfile.write(b"ok")

    def log_message(self, *args):
        pass


@pytest.fixture
def proxy():
    server = HTTPServer(("127.0.0.1", 0), _Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    yield f"http://127.0.0.1:{server.server_port}"
    server.shutdown()
    server.server_close()


def test_disabled_without_endpoint(monkeypatch):
    monkeypatch.delenv(FIXTURE_ENDPOINT_ENV, raising=False)
    client = FixtureClient()

    assert client.enabled is False
    assert client.healthy() is False
    with pytest.raises(FixtureError):
        client.call("get_orders", {"customer_id": 42})


def test_reads_endpoint_from_environment(monkeypatch, proxy):
    monkeypatch.setenv(FIXTURE_ENDPOINT_ENV, proxy)
    client = FixtureClient()

    assert client.enabled is True
    assert client.healthy() is True
    assert client.call("get_orders", {"customer_id": 42}) == [{"id": 123}]


def test_missing_fixture_raises(proxy):
    client = FixtureClient(endpoint=proxy)
    with pytest.raises(FixtureError, match="no fixture found"):
        client.call("unknown_tool", {})


def test_injected_failure_raises(proxy):
    client = FixtureClient(endpoint=proxy)
    with pytest.raises(FixtureError, match="injected simulated server error"):
        client.call("flaky_tool", {})


def test_unreachable_proxy_raises():
    client = FixtureClient(endpoint="http://127.0.0.1:1", timeout=1.0)
    with pytest.raises(FixtureError, match="unreachable"):
        client.call("get_orders", {})
