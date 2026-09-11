---
title: Invariants
nav_order: 6
parent: Architecture
---
# Invariants

Hard rules Gust must keep. Violating one is a bug, not a style preference.

## AgentRun is evidence, not a test

A captured `AgentRun` records what happened once. It is never automatically converted into assertions. Humans author `TestScenario` expectations independently so current bugs are not enshrined as the specification. `gust scenario from-run` (and `propose`) always leave `assertions: []` until a reviewer fills them in.

## Analyze ≠ Replay ≠ Test

| Mode | What it does | What it must not do |
|------|----------------|---------------------|
| **Analyze** | Evaluate assertions against existing evidence | Invoke an LLM or live agent |
| **Replay** | Reconstruct a recorded trajectory against fixtures | Re-decide tools or call a model |
| **Test** | Run the agent *N* times under controlled fixtures | Treat a single sample as reliability |

## Fixture namespace isolation (Mode 3)

Every Mode 3 sample **must** have an isolated fixture namespace:

```text
fixture namespace = evaluation_id + scenario_id + sample_id
```

When fixtures are clonable and a proxy factory is configured, Gust clones provider state and gives each sample its own HTTP endpoint. Concurrent samples must not observe each other's stateful fixture mutations (counters, ordered sequences, order status). Cross-sample contamination is a release-blocking defect for Mode 3.

Without clone+proxy, Gust falls back to serial execution with `Reset()` between samples.

## Hard vs soft assertions

- `criticality: hard` — sample failure; contributes to the Wilson denominator.
- `criticality: soft` — evidence only; does not fail the sample.
- Policy hard constraints (`forbidden_tools`, `schema_violations`) can fail the CI suite immediately when counts exceed caps.

## Execution errors and the denominator

Infrastructure / execution errors (trigger, timeout, fixture, ingest, evaluator) are tracked separately from behavioral failures. They are **excluded** from the Wilson pass/fail denominator but still counted toward `max_execution_error_rate`. Observed reliability is `passes / samples_completed` among non-infrastructure samples.

## Statistics stay honest

Do not widen intervals, weaken verdicts, or turn inconclusive `FLAKY` into `PASS` for convenience. Independence between samples is an assumption, not a guarantee (see [Statistics and policy]({% link architecture/statistics-and-policy.md %})).
