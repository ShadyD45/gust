import io
import json

from gust_sdk import EvaluatorPlugin, serve


class AlwaysPasses(EvaluatorPlugin):
    name = "always_passes"
    version = "2.1.0"
    description = "test double"
    capabilities = ["testing"]

    def evaluate(self, run, expected, context):
        return {"passed": True, "message": "ok"}


class RequiresTool(EvaluatorPlugin):
    name = "requires_tool"
    version = "1.0.0"

    def evaluate(self, run, expected, context):
        tool = (expected or {}).get("tool")
        if not tool:
            return {"passed": True, "message": "no tool specified"}
        spans = self.tool_spans(run, tool)
        if not spans:
            return {
                "passed": False,
                "score": 0.0,
                "message": f"{tool} was never called",
                "evidence": {"expected_tool": tool},
            }
        return {"passed": True, "evidence": {"call_count": len(spans)}}


class Explodes(EvaluatorPlugin):
    name = "explodes"

    def evaluate(self, run, expected, context):
        raise RuntimeError("kaboom")


RUN = {
    "schema_version": "0.5",
    "run_id": "r1",
    "trace": [
        {"span_id": "s1", "name": "get_orders", "type": "tool", "attributes": {"input": {"customer_id": 42}}},
        {"span_id": "s2", "name": "think", "type": "llm"},
    ],
    "outcome": {"status": "completed"},
}


def call(plugin, method, params=None, request_id=1):
    return plugin.handle({"jsonrpc": "2.0", "id": request_id, "method": method, "params": params})


def test_manifest_shape():
    manifest = call(AlwaysPasses(), "manifest")["result"]

    assert manifest["protocol_version"] == "1.0"
    assert manifest["kind"] == "evaluator"
    assert manifest["name"] == "always_passes"
    assert manifest["version"] == "2.1.0"
    assert manifest["capabilities"] == ["testing"]


def test_evaluate_fills_result_envelope():
    result = call(AlwaysPasses(), "evaluate", {"run": RUN, "expected": None, "context": {}})["result"]

    assert result["evaluator_name"] == "always_passes"
    assert result["evaluator_version"] == "2.1.0"
    assert result["passed"] is True
    assert result["score"] == 1.0
    assert result["execution_time_ns"] >= 0


def test_failing_evaluation_defaults_score_to_zero():
    params = {"run": RUN, "expected": {"tool": "cancel_order"}, "context": {}}
    result = call(RequiresTool(), "evaluate", params)["result"]

    assert result["passed"] is False
    assert result["score"] == 0.0
    assert result["evidence"] == {"expected_tool": "cancel_order"}


def test_nil_assertion_passes_vacuously():
    result = call(RequiresTool(), "evaluate", {"run": RUN, "expected": None})["result"]
    assert result["passed"] is True


def test_tool_span_helpers_filter_by_type():
    plugin = RequiresTool()
    assert len(plugin.tool_spans(RUN)) == 1
    assert plugin.tool_spans(RUN, "get_orders")[0]["span_id"] == "s1"
    assert plugin.tool_spans(RUN, "missing") == []
    assert plugin.tool_arguments(RUN["trace"][0]) == {"customer_id": 42}
    assert plugin.tool_arguments(RUN["trace"][1]) == {}


def test_unknown_method_returns_jsonrpc_error():
    response = call(AlwaysPasses(), "teleport")
    assert response["error"]["code"] == -32601


def test_plugin_exception_becomes_error_not_crash():
    response = call(Explodes(), "evaluate", {"run": RUN})
    assert response["error"]["code"] == -32603
    assert "kaboom" in response["error"]["message"]


def test_serve_round_trip_over_streams():
    requests = "\n".join(
        [
            json.dumps({"jsonrpc": "2.0", "id": 1, "method": "manifest"}),
            "",  # blank lines are ignored
            "not json",  # noise is ignored, stream survives
            json.dumps({"jsonrpc": "2.0", "id": 2, "method": "evaluate", "params": {"run": RUN}}),
        ]
    )
    stdout = io.StringIO()
    serve(AlwaysPasses(), stdin=io.StringIO(requests), stdout=stdout)

    responses = [json.loads(line) for line in stdout.getvalue().splitlines() if line]
    assert [r["id"] for r in responses] == [1, 2]
    assert responses[0]["result"]["name"] == "always_passes"
    assert responses[1]["result"]["passed"] is True
