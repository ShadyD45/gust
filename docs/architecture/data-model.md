---
title: Data model
nav_order: 4
parent: Architecture
---
# Data Model

Primary types live in `pkg/api`. JSON Schema contracts (wire/public shape) live under `spec/schemas/`. Go `Validate()` methods enforce semantic invariants beyond structural shape — keep both aligned for closed enums (assertion types, failure modes, etc.).

## Schema compatibility

`AgentRun.schema_version` is currently **`0.5`** (`api.SchemaVersion`). On the **0.x** line, Gust requires an **exact** match — unknown or older versions are rejected by `Validate()`.

Planned policy for a future **1.x** line:

- Readers accept the same major version (forward-compatible within the major).
- Writers emit one canonical version.
- A migrate helper may appear when the schema actually bumps; until then, treat `0.5` as the only supported wire version.

`max_steps` counts top-level `tool` and `agent` spans only (not nested `llm` / `retrieval` sub-steps).

## AgentRun

Captured or synthesized execution trace:

- `run_id`, `agent`, `task`, `trace[]` (`Span`), `outcome`, optional `metadata`
- Spans carry type (`agent`, `llm`, `tool`, …), timing, attributes, status

## Fixture

Recorded or authored tool response used for Replay / Test mocking:

- `fixture_id`, `tool`, `input_hash`, match strategy, recorded I/O
- Failure modes: `success`, `timeout`, `malformed`, `slow`, `partial_failure`, `recorded_error` (captured from a real error span)
- Provenance: `recorded` | `authored`

Match strategies: exact hash, ordered sequence (FIFO), hybrid (prefer exact then sequence).

## TestScenario

The real test artifact:

- `task` — what to ask the agent
- `environment.fixtures` / `fixtures_dir` — controlled world (bodies usually live as JSON files)
- `assertions` / `assertion_files` / `$ref` — independent expectations (never auto-copied from a buggy trace)
- `reliability` — samples, minimum pass rate, confidence
- `provenance` — extraction metadata and human review fields

## Assertion

Typed checks (`tool_call`, `forbidden_tool_call`, `max_steps`, …) with optional `criticality` (`hard` | `soft`).

## Policy

CI acceptance gate:

- Hard constraints: max allowed failed `forbidden_tool` / `schema_validation` evaluations (`0` = zero tolerance). Those named counts fail CI immediately when exceeded. Other assertions with `criticality: hard` fail the sample (Wilson); `criticality: soft` is recorded but does not fail the sample.
- Reliability defaults (`default_minimum_pass_rate` applied when a scenario omits `minimum_pass_rate`), `on_flaky`, optional `max_execution_error_rate` (omit → 0.20; `0` → zero tolerance), optional `retry`
- Regression thresholds (pass-rate drop, latency increase). Unmeasurable baseline latency omits `latency_increase_ratio` rather than reporting `0.0`.

Project-level execution knobs (`concurrency`, timeouts, retry) live in `gust.yaml`. Local filesystem caches for scenarios/runs are mutable by ID; content-addressed immutability is `gust dataset bundle` / `verify`.

## ReliabilityResult

Mode 3 aggregate:

- `scenario_id`, `samples`, `passes`, `observed_pass_rate`
- `confidence_interval` `[lower, upper]`
- `verdict`, optional `per_run_evidence`

## Content addressing

`pkg/jcs` implements RFC 8785 JSON Canonicalization Scheme. Content hashes (SHA-256 over canonical bytes) identify fixtures and dataset manifests so comparisons are bit-for-bit stable across languages and formatters.

`gust dataset bundle <dir> --id <name>` writes scenario JSON plus `dataset.json`. Re-bundling the same ID with **different** content is a hard error unless `--force`; orphans are pruned only with `--force`. `gust dataset verify <dir>` checks hashes.

