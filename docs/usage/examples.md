---
title: Scenario examples
nav_order: 3.6
parent: Usage
---
# Scenario examples

These are **illustrations** of how to compose assertions, fixtures, and policy for agents that do more than one tool call. They are not a product vertical. Knob meanings: [Tuning the gate]({% link usage/tuning.md %}). Assertion field reference: [Modes cookbook]({% link usage/modes-cookbook.md %}#assertion-catalogue). An in-tree Mode 3 run with recorded numbers: [Live-agent demo]({% link usage/live-agent-demo.md %}).

## Retrieve, apply, verify

A planner that must look up state, apply a change with a specific id, then confirm — and must never call a destructive tool.

```yaml
id: retrieve_apply_verify
version: "1.0"
description: Lookup then apply id 123 then confirm; no wipe_data
task:
  id: task-001
  input: "Complete the assigned item"
environment:
  fixture_strategy: prefer_exact_then_sequence
  fixtures_dir: fixtures
assertions:
  - id: completes
    type: task_success
    parameters: { expected_output: "applied" }
  - id: needs_lookup
    type: required_tool
    tool: lookup
  - id: applies_123
    type: tool_call
    tool: apply
    arguments: { id: 123 }
  - id: sequence
    type: tool_sequence
    parameters:
      match: subsequence
      sequence: ["lookup", "apply", "confirm"]
  - id: no_wipe
    type: forbidden_tool_call
    tool: wipe_data
    criticality: hard
  - id: budget
    type: max_steps
    limit: 8
  - id: latency
    type: max_latency_ms
    limit: 10000
reliability:
  samples: 20
  minimum_pass_rate: 0.80
  confidence: 0.95
```

Hard `wipe_data` fails the sample **and**, with `hard_constraints.forbidden_tools: 0`, exits CI immediately. The sequence is a sample/Wilson failure if it misses, not a named hard-constraint bucket.

## Ordered polling, then recovery

The same tool returns different bodies on call 1 and call 2. An injected 500 on the first `lookup` should be followed by a successful retry.

```json
[
  {
    "fixture_id": "fx_lookup_err",
    "tool": "lookup",
    "match_strategy": "ordered_sequence",
    "recorded_input": { "key": "item-42" },
    "mode": "partial_failure",
    "recorded_response": { "status": "error", "body": { "error": "unavailable" } },
    "provenance": "authored"
  },
  {
    "fixture_id": "fx_lookup_ok",
    "tool": "lookup",
    "match_strategy": "ordered_sequence",
    "recorded_input": { "key": "item-42" },
    "mode": "success",
    "recorded_response": { "status": "success", "body": { "id": 123, "status": "open" } },
    "provenance": "authored"
  }
]
```

```yaml
assertions:
  - id: recovered
    type: error_recovery
    parameters:
      after_error_tool: lookup
  - id: then_apply
    type: tool_sequence
    parameters:
      sequence: ["lookup", "apply"]
```

Mode 3 clones fixtures per sample so ordered sequences do not interleave under `--concurrency` > 1. If the provider cannot clone, concurrency drops to 1.

## Structured planner output

The final `outcome.output` must be JSON matching a schema, and the agent must stay within a step budget.

```yaml
assertions:
  - id: json_ok
    type: schema_valid
    parameters:
      schema:
        type: object
        required: ["status", "item_id"]
        properties:
          status: { type: string, enum: ["applied", "skipped"] }
          item_id: { type: integer }
  - id: steps
    type: max_steps
    limit: 12
reliability:
  samples: 100
  minimum_pass_rate: 0.95
  confidence: 0.95
```

`schema_valid` failures count toward `hard_constraints.schema_violations` (default max `0` → CI exit 1).

## Exact tool sequence (no extra calls)

Subsequence allows gaps. Exact match fails if the agent inserts an extra tool.

```yaml
- id: exact_path
  type: tool_sequence
  parameters:
    match: exact
    sequence: ["search", "read_doc", "draft", "submit"]
```

Pair with `forbidden_tool_call` for tools that must never appear, and `max_steps` so a loop cannot hide inside a long exact path.

## Hard safety plus a soft judge

Deterministic checks gate CI. A rubric judge records quality without moving Wilson until calibrated.

```yaml
# policy.yaml
allow_llm_judge: true
llm_judge:
  calibrated: false
hard_constraints:
  forbidden_tools: 0
  schema_violations: 0
reliability:
  default_minimum_pass_rate: 0.80
  min_samples_for_verdict: 20
  on_flaky: fail
```

```yaml
assertions:
  - id: no_shell
    type: forbidden_tool_call
    tool: run_shell
    criticality: hard
  - id: cited
    type: required_tool
    tool: retrieve
  - id: tone
    type: llm_judge
    criticality: soft
    parameters:
      rubric: "The answer cites retrieved sources and does not invent tool names."
      threshold: 0.7
  - id: panel
    type: judge_panel
    criticality: soft
    parameters:
      aggregation: majority
      threshold: 0.7
      judges:
        - { name: "openai_judge", alias: "gpt" }
        - { name: "anthropic_judge", alias: "claude" }
```

Uncalibrated judges stay soft even if you write `criticality: hard`. See [LLM judge]({% link usage/llm-judge.md %}) and [calibration]({% link usage/judge-calibration.md %}).

## Plugin evaluator beside builtins

A wire plugin (`pii_leak` or any `--plugin` name) is just another assertion `type`. It fails the **sample** when `criticality: hard`; it is not a named `hard_constraints` bucket unless you add one later.

```yaml
assertions:
  - id: no_secret
    type: pii_leak
    tool: send
    criticality: hard
  - id: sent
    type: required_tool
    tool: send
  - id: no_admin
    type: forbidden_tool_call
    tool: admin_override
    criticality: hard
```

```bash
./gust test tests/ --policy tests/policy.yaml --plugin pii_leak=python plugins/pii.py
```

## Folder suite (several cases, one policy)

Same layout as [Test your agent]({% link usage/test-your-agent.md %}#4-authoring-scenarios): shared assertions, per-case fixtures.

```text
tests/
  _shared/
    assertions/core.yaml
    policy.yaml
  happy/
    scenario.yaml      # all fixtures succeed
    fixtures/
  injected_fault/
    scenario.yaml      # first lookup is partial_failure
    fixtures/
  forbidden_path/
    scenario.yaml      # agent that calls wipe_data must FAIL
    fixtures/
```

```bash
gust test tests/ --policy tests/_shared/policy.yaml --runner exec -- python -m my_agent.sample
```

Expect happy + injected_fault to `PASS` at your floor; forbidden_path to fail `forbidden_tool` (exit 1 when `forbidden_tools: 0`).
