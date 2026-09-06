"""Client for the gust mock tool proxy.

During Mode 3 sampling, gust starts an ephemeral HTTP proxy that answers tool
calls from fixtures. Routing your tool layer through this client means tests
exercise the real agent against controlled dependencies, never production
systems.
"""

from __future__ import annotations

import json
import os
import urllib.error
import urllib.request
from typing import Any, Dict, Optional

FIXTURE_ENDPOINT_ENV = "AGENTEVAL_FIXTURE_ENDPOINT"


class FixtureError(RuntimeError):
    """Raised when the proxy has no fixture for a call, or cannot be reached."""


class FixtureClient:
    """Calls tools through the gust fixture proxy instead of the real service.

    ``endpoint`` defaults to ``AGENTEVAL_FIXTURE_ENDPOINT``. When that variable
    is unset, :attr:`enabled` is ``False`` and your code should call the real
    tool — that is how the same agent code runs in production and under test.
    """

    def __init__(self, endpoint: Optional[str] = None, timeout: float = 30.0) -> None:
        self.endpoint = (endpoint or os.environ.get(FIXTURE_ENDPOINT_ENV, "")).rstrip("/")
        self.timeout = timeout

    @property
    def enabled(self) -> bool:
        return bool(self.endpoint)

    def call(self, tool: str, arguments: Optional[Dict[str, Any]] = None) -> Any:
        """Resolve a tool call against fixtures and return the response body.

        Injected failures (500s, malformed bodies, timeouts) surface as
        :class:`FixtureError` or malformed data, which is the point: they
        exercise your agent's error handling.
        """
        if not self.enabled:
            raise FixtureError(
                f"no fixture endpoint configured; set {FIXTURE_ENDPOINT_ENV} or pass endpoint="
            )

        payload = json.dumps({"tool": tool, "arguments": arguments or {}}).encode("utf-8")
        request = urllib.request.Request(
            f"{self.endpoint}/v1/tools/call",
            data=payload,
            headers={"Content-Type": "application/json"},
            method="POST",
        )

        try:
            with urllib.request.urlopen(request, timeout=self.timeout) as response:
                body = json.loads(response.read().decode("utf-8"))
        except urllib.error.HTTPError as exc:
            detail = exc.read().decode("utf-8", errors="replace")
            raise FixtureError(f"fixture proxy returned {exc.code} for {tool!r}: {detail}") from exc
        except urllib.error.URLError as exc:
            raise FixtureError(f"fixture proxy unreachable at {self.endpoint}: {exc}") from exc

        if body.get("status") == "error":
            raise FixtureError(body.get("error") or f"fixture error for tool {tool!r}")
        return body.get("body")

    def healthy(self) -> bool:
        """Return True when the proxy answers its health check."""
        if not self.enabled:
            return False
        try:
            with urllib.request.urlopen(f"{self.endpoint}/health", timeout=self.timeout) as resp:
                return resp.status == 200
        except (urllib.error.URLError, OSError):
            return False
