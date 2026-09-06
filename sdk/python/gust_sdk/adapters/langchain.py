"""LangChain callback that records tool and LLM spans onto a RunRecorder.

Install the extra::

    pip install 'gust-sdk[langchain]'

Then attach :class:`GustCallbackHandler` to your agent or LLM.
"""

from __future__ import annotations

from typing import Any, Dict, Optional
from uuid import UUID

from gust_sdk.recorder import RunRecorder

try:
    from langchain_core.callbacks import BaseCallbackHandler
except ImportError:  # pragma: no cover - adapter is optional
    BaseCallbackHandler = object  # type: ignore[misc,assignment]


class GustCallbackHandler(BaseCallbackHandler):
    """Maps LangChain callback events onto a gust :class:`RunRecorder`."""

    def __init__(
        self,
        recorder: Optional[RunRecorder] = None,
        agent_name: str = "langchain-agent",
        agent_version: str = "0.1",
        task_input: str = "",
        task_id: Optional[str] = None,
    ) -> None:
        super().__init__()
        self.agent_name = agent_name
        self.agent_version = agent_version
        self._task_input = task_input
        self._task_id = task_id
        self.recorder = recorder
        self._open: Dict[str, Any] = {}

    def ensure_recorder(self, task_input: str = "") -> RunRecorder:
        if self.recorder is None:
            self.recorder = RunRecorder(
                agent_name=self.agent_name,
                agent_version=self.agent_version,
                task_input=task_input or self._task_input or "langchain run",
                task_id=self._task_id,
            )
        return self.recorder

    def on_tool_start(self, serialized: Dict[str, Any], input_str: str, *, run_id: UUID, **kwargs: Any) -> None:
        rec = self.ensure_recorder()
        name = serialized.get("name") or serialized.get("id") or "tool"
        arguments: Any = input_str
        if isinstance(input_str, str) and input_str[:1] in "{[":
            import json

            try:
                arguments = json.loads(input_str)
            except json.JSONDecodeError:
                arguments = {"input": input_str}
        elif not isinstance(arguments, dict):
            arguments = {"input": input_str}
        handle = rec.tool(str(name), arguments if isinstance(arguments, dict) else {"input": arguments})
        self._open[str(run_id)] = handle

    def on_tool_end(self, output: Any, *, run_id: UUID, **kwargs: Any) -> None:
        handle = self._open.pop(str(run_id), None)
        if handle is None:
            return
        handle.output = getattr(output, "content", output)
        handle._finish(None)

    def on_tool_error(self, error: BaseException, *, run_id: UUID, **kwargs: Any) -> None:
        handle = self._open.pop(str(run_id), None)
        if handle is None:
            return
        handle._finish(error)

    def on_llm_start(self, serialized: Dict[str, Any], prompts: Any, *, run_id: UUID, **kwargs: Any) -> None:
        rec = self.ensure_recorder()
        name = serialized.get("name") or serialized.get("id") or "llm"
        model = (serialized.get("kwargs") or {}).get("model") or kwargs.get("invocation_params", {}).get("model")
        handle = rec.llm(str(name), model=model)
        self._open[str(run_id)] = handle

    def on_llm_end(self, response: Any, *, run_id: UUID, **kwargs: Any) -> None:
        handle = self._open.pop(str(run_id), None)
        if handle is None:
            return
        text = getattr(response, "content", None)
        if text is None and hasattr(response, "generations"):
            try:
                text = response.generations[0][0].text
            except Exception:
                text = str(response)
        handle.output = text
        handle._finish(None)

    def on_llm_error(self, error: BaseException, *, run_id: UUID, **kwargs: Any) -> None:
        handle = self._open.pop(str(run_id), None)
        if handle is None:
            return
        handle._finish(error)

    def finish(self, output: str = "") -> Dict[str, Any]:
        rec = self.ensure_recorder()
        if rec._outcome is None:
            rec.complete(output=output)
        return rec.to_dict()
