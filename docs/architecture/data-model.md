# Data Model

Primary types live in `pkg/api`. JSON Schema contracts (documentation) live under `spec/schemas/`.

## AgentRun

Captured or synthesized execution trace:

- `run_id`, `agent`, `task`, `trace[]` (`Span`), `outcome`, optional `metadata`
- Spans carry type (`agent`, `llm`, `tool`, …), timing, attributes, status

## Fixture

Recorded or authored tool response used for Replay / Test mocking:

- `fixture_id`, `tool`, `input_hash`, match strategy, recorded I/O
- Failure modes: `success`, `timeout`, `malformed`, `slow`, `partial_failure`
- Provenance: `recorded` | `authored`

Match strategies: exact hash, ordered sequence (FIFO), hybrid (prefer exact then sequence).

## TestScenario

The real test artifact:

- `task` — what to ask the agent
- `environment.fixtures` — controlled world
- `assertions` — independent expectations (never auto-copied from a buggy trace)
- `reliability` — samples, minimum pass rate, confidence
- `provenance` — extraction metadata and human review fields

## Assertion

Typed checks (`tool_call`, `forbidden_tool_call`, `max_steps`, …) with optional `criticality` (`hard` | `soft`).

## Policy

CI acceptance gate:

- Hard constraints (e.g. forbidden tools = 0)
- Reliability defaults and `on_flaky` (`warn` | `fail` | `ignore`)
- Regression thresholds (pass-rate drop, latency increase)

## ReliabilityResult

Mode 3 aggregate:

- `scenario_id`, `samples`, `passes`, `observed_pass_rate`
- `confidence_interval` `[lower, upper]`
- `verdict`, optional `per_run_evidence`

## Content addressing

`pkg/jcs` implements RFC 8785 JSON Canonicalization Scheme. Content hashes (SHA-256 over canonical bytes) identify fixtures and dataset manifests so comparisons are bit-for-bit stable across languages and formatters.
