from uuid import uuid4

from gust_sdk.adapters.langchain import GustCallbackHandler
from gust_sdk.recorder import RunRecorder


def test_tool_callbacks_record_spans():
    rec = RunRecorder(agent_name="lc", agent_version="1", task_input="Cancel")
    handler = GustCallbackHandler(recorder=rec)
    rid = uuid4()
    handler.on_tool_start({"name": "get_orders"}, '{"customer_id": 42}', run_id=rid)
    handler.on_tool_end([{"id": 123, "status": "PROCESSING"}], run_id=rid)
    run = handler.finish(output="done")
    assert run["trace"][0]["name"] == "get_orders"
    assert run["trace"][0]["attributes"]["input"] == {"customer_id": 42}
    assert run["outcome"]["status"] == "completed"


def test_tool_error_is_recorded():
    rec = RunRecorder(agent_name="lc", agent_version="1", task_input="Cancel")
    handler = GustCallbackHandler(recorder=rec)
    rid = uuid4()
    handler.on_tool_start({"name": "cancel_order"}, '{"order_id": 1}', run_id=rid)
    handler.on_tool_error(RuntimeError("500"), run_id=rid)
    rec.fail("tool failed")
    span = rec.to_dict()["trace"][0]
    assert span["status"]["code"] == "error"
