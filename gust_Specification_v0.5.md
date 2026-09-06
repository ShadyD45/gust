# gust: Test Infrastructure for Autonomous Software

## Design, Requirements, Phased Roadmap, and Verification Plan

**Status:** v0.5 (supersedes v0.4)  
**Project type:** Open-source infrastructure/library  
**Reference implementation language:** Go, from Phase 0 (unchanged)  
**License:** Apache-2.0  
**Audience for this document:** Intended to be handed directly to an implementation team/coding agent for planning and build-out. Where a section says "MVP," treat it as literally in scope for the first buildable milestone; everything else is sequenced but deferred.

> **What's New in v0.5:** v0.4 established the foundational separation of the three execution modes (Analyze, Replay, Test) and elevated probabilistic repeated-sample testing into the MVP. v0.5 refines the engineering rigor for production implementation:
>
> 1. **Stateful vs. Stateless Fixture Resolution (§11.1, §15.1):** Solves polling and repetitive tool calls via ordered FIFO sequence queues alongside canonical hash lookup.
> 2. **Universal Tool Proxy / Mock Server Protocol (§9.3):** Defines how live agents (Go, Python, TypeScript, etc.) route tool calls to gust in Test mode via a local Mock Server supporting HTTP/JSON-RPC and Model Context Protocol (MCP).
> 3. **Mathematical Rigor & Edge Cases for Wilson Score Intervals (§13.2):** Full closed-form specification with clamping, continuity handling, and zero-sample / extreme-rate behavior.
> 4. **RFC 8785 Canonical JSON Scheme (JCS) (§15):** Guarantees cross-language bit-for-bit deterministic hashing for content-addressed fixtures and datasets.
> 5. **Mutator Lifecycle & Applicability (§9.2):** Explicitly handles non-applicable mutations (`Mutated`, `Skipped`, `Error`) to prevent distortion of Detection Rates.
> 6. **Assertion Criticality & Hard Constraint Gating (§7.4, §14):** Differentiates between soft probabilistic rates and hard zero-tolerance violations (e.g., forbidden safety tools).
> 7. **Go Idioms & Context Propagation (§9.1):** Thread-safe `context.Context`, cancellation, streaming, and error handling across all core interfaces.
>
> Full changelog in §21.

---

# 1. Executive Summary

Agent systems are software systems with planning, tool use, retrieval, memory, retries, delegation, and multi-step execution. The right mental model for gust is not "an LLM evaluation library." It is:

> **Test infrastructure for autonomous software** — the same role JUnit + a mocking framework + a CI regression gate plays for ordinary backend systems, adapted for a component (the LLM) that does not give the same answer twice.

Traditional software testing looks like:

```text
source code -> unit tests -> integration tests -> deterministic execution -> assertions -> CI -> regression detection
```

The agent equivalent this project builds is:

```text
agent -> production execution -> something went wrong -> capture AgentRun ->
reconstruct a reproducible scenario -> turn it into a regression test ->
run the REAL agent against a controlled environment, repeatedly -> CI
```

Three ideas anchor the whole design:

1. **A trace is not the test.** A captured `AgentRun` is evidence — it tells you what happened once. A `TestScenario` (input + a controlled environment + independent assertions about correct behavior) is the actual test artifact. Building a fancy trace replayer instead of this is the single biggest way this project could go wrong (§4.1).
2. **Three distinct execution modes exist, and the spec must never blur them: Analyze, Replay, and Test (§5).** Only one of them — Test — invokes a real agent with a real LLM, and it is the only one that tells you whether *today's* agent is still correct.
3. **Because the LLM is not deterministic, a single Test-mode execution proves very little. Probabilistic, repeated-sample assertions (§13) are therefore MVP scope, not a future enhancement** — a test suite that reports `PASS`/`FAIL` from one sample when the true pass rate is 85% is not testing infrastructure, it's a coin flip with extra steps.

Mutation testing (§12) remains the project's flagship trust metric for the deterministic parts of the stack (evaluators, policy aggregation) — it operates in Replay mode, against synthetic or recorded agents, and does not require the probabilistic machinery in §13, which exists specifically for Test-mode's live-LLM nondeterminism.

---



# 2. Why This Project Exists



## 2.1 Agent behavior isn't captured by "was the final answer good"

An agent may call the wrong tool, use invalid arguments, repeat a tool unnecessarily, loop, fail to recover from an error, violate a plan, incur excessive latency/cost, access an inappropriate tool, or succeed only accidentally — none of which a final-answer-only check would catch.

## 2.2 And even a trajectory-level check is not the whole story, because the decision-maker is stochastic

Suppose a regression test asserts the agent calls `cancel_order(order_id=123)`. Run it once, it passes. Run it five times:

```text
Run 1 -> PASS
Run 2 -> PASS
Run 3 -> FAIL
Run 4 -> PASS
Run 5 -> PASS
```

A traditional unit test that behaved this way would be considered broken. An agent test that behaves this way is normal, and pretending otherwise (by running it once and trusting the result) is the single most common way agent test suites lie to their users. gust's job is to measure and report the *rate*, honestly, not to paper over it with a single boolean (§13).

## 2.3 Replaying old mistakes is not testing

If a production trace recorded `cancel_order(122)` (the wrong order), and a "regression test" simply replays that exact call and checks it still happens, it has enshrined the bug as expected behavior. The correct regression test asks the **new agent to independently decide**, given the same *environment*, and checks the *independently made* decision against the *correct* expected behavior — not against what happened before (§4.1, §13.1).

---



# 3. Competitive Landscape & Real Differentiation

Named competitors and their real capabilities:


| Project       | Strong at                                                                                                   | Still doesn't make first-class                                                                                                                                                                     |
| ------------- | ----------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| DeepEval      | Trajectory/component agent metrics, tracing, CI/CD                                                          | Mutation-validating its own evaluators; a formal recorded-environment protocol; probabilistic pass-rate assertions as a primary testing primitive rather than a single run.                        |
| Braintrust    | Datasets, immutable experiments, CI regression, production feedback loops; recommends stubbing dependencies | The stubbing recommendation is a *pattern*, not a schema-backed `TestScenario`/`Environment` protocol with mutation validation and reliability sampling built in.                                  |
| Arize Phoenix | Deterministic + LLM evaluators, OTel-native, fast batch evaluation                                          | Same gap: no first-class distinction between "replaying a recorded trace" and "testing a live agent against a controlled environment," and no built-in reliability-under-repeated-sampling metric. |




### Undefended territory (Six Pillars):

1. **Mutation-testing the evaluator suite itself** (Detection Rate + False Positive Rate).
2. **A formal, versioned recorded-fixture/replay protocol** with stateful sequence support and failure injection.
3. **A policy engine as the actual pass/fail authority**, decoupled from any individual evaluator, handling soft pass-rate thresholds and hard security constraints.
4. **A cross-language wire protocol** (JSON-RPC/stdio) for evaluators, mutators, fixture providers, and test runners.
5. **Benchmarked deterministic-evaluation throughput** (≥1,000 cases/sec on a single core).
6. **A first-class, explicit separation between "analyzing a past execution," "deterministically replaying a recorded execution," and "testing a live agent's independent decision-making,"** with the third treated as inherently probabilistic and measured with repeated sampling and Wilson score confidence intervals.

---



# 4. Product Vision & Core Principles



## 4.1 The Core Principle: Trace is not the test

> **A trace is not the test. A trace is evidence from which a reproducible test scenario can be created.**

- `AgentRun` (a captured execution) and `TestScenario` (a reusable test artifact, §7) are **different objects with different schemas** — one is a record of past actions, the other is a specification of expected future behavior.
- Converting a production `AgentRun` into a `TestScenario` is an assisted-but-human-reviewed transformation (§16).
- A `TestScenario`'s environment can vary from what was recorded (e.g., two orders instead of one) to verify generalized reasoning.



## 4.2 Language & Framework Agnostic

Any agent should be able to produce an gust-compatible execution, regardless of model provider, framework (LangChain, LlamaIndex, AutoGen, CrewAI, Custom), language, or deployment.

## 4.3 Long-Term Architecture

```text
                             gust Standard
                                     |
                      +--------------+--------------+
                      |                             |
                  AgentRun /                    Evaluator
               TestScenario Schema               Protocol
                      |                             |
                      +--------------+--------------+
                                     |
                              gust Core (Go)
                                     |
         +------------+------------+------------+------------+
         |            |            |            |            |
      Analyze       Replay        Test        Mutate       Policy
     (no exec,   (deterministic  (REAL agent,  (inject     (aggregate
      evaluate    re-execution,   REAL LLM,    known         evidence,
      captured    no live LLM,    controlled   defects into  issue
      AgentRun)   for debugging/  environment,  synthetic/   verdict —
                  engine bench)   probabilistic replayed     PASS/FAIL/
                                  sampling)    agents,       FLAKY)
                                               Detect+FPR)
         |            |            |            |            |
         +------------+------------+------------+------------+
                                     |
                                     v
                           Regression Result
                      (structured evidence, §19;
                       pass rate + confidence
                       interval where probabilistic)
                                     |
                      +--------------+--------------+
                      |                             |
                     CI                        Production
```

---



# 5. The Three Execution Modes

The spec enforces three architecturally isolated execution modes:

```text
+-----------------------------------------------------------------------------------+
| Mode 1: ANALYZE                                                                  |
| Existing AgentRun -> Evaluators -> EvaluationResult -> Policy -> Verdict         |
| (No execution, no agent, no LLM, 100% deterministic, zero network)               |
+-----------------------------------------------------------------------------------+
| Mode 2: REPLAY                                                                    |
| Existing AgentRun -> Fixture Store -> Deterministic Re-execution -> New AgentRun  |
| (Mocked/recorded LLM decisions, mocked tools, 100% deterministic, zero network)  |
+-----------------------------------------------------------------------------------+
| Mode 3: TEST (The Regression Testing Primitive)                                   |
| TestScenario -> REAL Agent -> REAL LLM -> Fixture Store (Mocked Tools)            |
| -> Repeated N Times -> Sampled Runs -> Evaluators -> Policy -> Statistical Report|
| (Live agent decision-making, controlled tool environment, probabilistic)          |
+-----------------------------------------------------------------------------------+
```



### Key Distinctions:

- **Analyze** consumes frozen history to produce diagnostic evidence.
- **Replay** reproduces control flow deterministically against recorded fixtures to benchmark evaluators and run mutation tests.
- **Test** executes the real agent and real LLM against mocked tools across $N$ samples to measure regression and reliability.

---



# 6. Goals & Scope



## P0 Goals (MVP)

1. **Domain Models & Schemas:** `AgentRun`, `Span`, `Fixture`, `TestScenario`, `Assertion`, `Policy`, `EvaluationResult`, `ReliabilityResult`, `MutationResult`.
2. **Canonical Serialization:** RFC 8785 JSON Canonicalization Scheme (JCS) with SHA-256 for deterministic content addressing.
3. **Pluggable Architecture:** Tier 1 (native Go) and Tier 2 (stdio/JSON-RPC wire protocol) for Evaluators, Mutators, FixtureProviders, and TestRunners.
4. **Deterministic Evaluator Suite:** 10 core evaluators running with zero LLM dependence.
5. **Mode 1 (Analyze) & Mode 2 (Replay):** Fully functional with content-addressed Fixture Store and failure-injection modes.
6. **Mode 3 (Test) & Probabilistic Sampling:** Parallel test runner execution, Wilson score confidence interval calculation, and `PASS`/`FAIL`/`FLAKY`/`INSUFFICIENT_SAMPLES` classification.
7. **Tool Proxy / Fixture Server:** Built-in HTTP and JSON-RPC mock server so live agents can intercept tool calls in Test mode.
8. **Mutation Engine:** 9 mutation classes measuring Detection Rate (≥90%) and False Positive Rate (≤5%) in Replay mode.
9. **Policy Engine:** Evaluation of hard constraints, soft reliability thresholds, and regression comparison between baseline and candidate.
10. **Assisted Scenario Extraction:** `gust scenario from-run` extracting inputs and environment without auto-populating assertions.
11. **Single Static Binary CLI:** `analyze`, `replay`, `test`, `mutate`, `compare`, and `scenario` subcommands.
12. **Offline-First:** All modes operate offline with zero cost, using a local model (Ollama) for Test mode.



## Non-Goals (MVP)

- No agent runtime or execution framework.
- No hosted web dashboard or cloud account requirement.
- No LLM-as-judge evaluator in MVP (deferred to P1).
- No single-sample ($n=1$) pass/fail conclusions for Test mode.
- No auto-generation of test assertions from captured runs without human review.

---



# 7. Core Domain Model & JSON Schemas

All schemas are formalized under `spec/schemas/` using JSON Schema Draft 2020-12.

## 7.1 AgentRun

Represents a completed execution trace.

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "AgentRun",
  "type": "object",
  "required": ["schema_version", "run_id", "agent", "task", "trace", "outcome"],
  "properties": {
    "schema_version": { "type": "string", "enum": ["0.5"] },
    "run_id": { "type": "string" },
    "agent": {
      "type": "object",
      "required": ["name", "version"],
      "properties": {
        "name": { "type": "string" },
        "version": { "type": "string" },
        "git_commit": { "type": "string" }
      }
    },
    "task": {
      "type": "object",
      "required": ["id", "input"],
      "properties": {
        "id": { "type": "string" },
        "input": { "type": "string" },
        "context": { "type": "object" }
      }
    },
    "trace": {
      "type": "array",
      "items": { "$ref": "#/$defs/Span" }
    },
    "outcome": {
      "type": "object",
      "required": ["status"],
      "properties": {
        "status": { "type": "string", "enum": ["completed", "failed", "timeout", "cancelled"] },
        "output": { "type": "string" },
        "error": { "type": "string" }
      }
    },
    "metadata": { "type": "object" }
  }
}
```



## 7.2 Fixture (with Stateful & Failure Support)

```json
{
  "fixture_id": "fx_order_status_poll_01",
  "tool": "check_order_status",
  "input_hash": "sha256:4a8b...canonical_jcs_hash...",
  "match_strategy": "ordered_sequence",
  "sequence_order": 1,
  "recorded_input": { "order_id": 123 },
  "recorded_response": {
    "status": "success",
    "status_code": 200,
    "body": { "id": 123, "state": "PROCESSING" }
  },
  "mode": "success",
  "delay_ms": 50,
  "provenance": "recorded"
}
```



## 7.3 TestScenario

```yaml
id: cancel_latest_order
version: "1.0"
description: >
  Given two customer orders where the most recent is processing and an older one
  is delivered, the agent must cancel the processing order and must not cancel the delivered one.

task:
  input: "Please cancel my latest order."
  context:
    user_id: "cust_992"

environment:
  fixture_strategy: "prefer_exact_then_sequence"
  fixtures:
    - fixture_id: fx_get_orders
      tool: get_orders
      recorded_response:
        body:
          - { id: 122, date: "2026-05-01", status: "DELIVERED" }
          - { id: 123, date: "2026-06-01", status: "PROCESSING" }
    - fixture_id: fx_cancel_123
      tool: cancel_order
      recorded_input: { order_id: 123 }
      recorded_response:
        body: { success: true, cancelled_id: 123 }

assertions:
  - id: assert_correct_cancel
    type: tool_call
    criticality: hard
    tool: cancel_order
    arguments:
      order_id: 123

  - id: assert_no_delivered_cancel
    type: forbidden_tool_call
    criticality: hard
    tool: cancel_order
    arguments:
      order_id: 122

  - id: assert_step_budget
    type: max_steps
    criticality: soft
    limit: 6

reliability:
  samples: 20
  minimum_pass_rate: 0.95
  confidence: 0.95

provenance:
  source: production_incident
  source_run_id: run_prod_88192
  extracted_at: "2026-06-02T10:15:00Z"
  reviewed_by: "dev@example.com"
```



## 7.4 Assertion Model

An `Assertion` defines an expectation. Each assertion has a `criticality`:

- `hard`: If failed in any sample run, that run immediately fails. Furthermore, if configured in policy, a hard violation in any run can immediately fail the whole test scenario or CI gate.
- `soft`: Counts toward the run's success/failure for calculating the overall sample pass rate.

---



# 8. Span Types & Trajectory Model

Supported span types in `AgentRun.trace`:

- `agent`: Root execution span.
- `llm`: Interaction with language model (prompts, completion, token counts, temperature).
- `tool`: Execution of external tool/function (tool name, input arguments, output/error).
- `retrieval`: RAG retrieval query and documents returned.
- `memory`: State read/write operations.
- `plan`: Plan generation, revision, or decomposition steps.
- `error`: Explicit exception or failure event.

Every span contains: `span_id`, `parent_span_id`, `name`, `type`, `start_time`, `end_time`, `attributes`, and `status`.

---



# 9. Evaluator, Mutator, Fixture & TestRunner Interfaces

All Go interfaces adhere to standard idioms: thread-safe, accept `context.Context`, use value returns for results, and return typed errors.

## 9.1 Native Go Interfaces (Tier 1)

```go
package core

import "context"

// Evaluator checks an AgentRun against expected behaviors.
type Evaluator interface {
    Name() string
    Version() string
    Evaluate(ctx context.Context, run AgentRun, expected *ExpectedBehavior, evalCtx EvaluationContext) (EvaluationResult, error)
}

// Mutator injects a defect into an AgentRun for mutation testing.
type Mutator interface {
    Name() string
    Class() MutationClass
    Mutate(ctx context.Context, run AgentRun) (MutationOutcome, error)
}

// FixtureProvider resolves recorded tool responses during Replay and Test modes.
type FixtureProvider interface {
    Lookup(ctx context.Context, call ToolCall) (RecordedResponse, bool, error)
    Record(ctx context.Context, call ToolCall, resp RecordedResponse) error
}

// TestRunner executes a live agent against a controlled environment.
type TestRunner interface {
    Name() string
    Run(ctx context.Context, scenario TestScenario, fixtureEndpoint string) (AgentRun, error)
}
```



## 9.2 MutationOutcome Contract

To prevent skewing the mutation benchmark when a mutant is inapplicable to a specific trace:

```go
type MutationStatus string

const (
    MutationApplied MutationStatus = "applied"
    MutationSkipped MutationStatus = "skipped"
    MutationError   MutationStatus = "error"
)

type MutationOutcome struct {
    Status      MutationStatus
    OriginalRun AgentRun
    MutatedRun  AgentRun
    SkipReason  string
    Description string
}
```



## 9.3 Tool Proxy / Mock Server Protocol

In Mode 3 (Test), the real agent runs with a real LLM. To ensure tool calls hit the `FixtureProvider` rather than live production APIs:

1. gust starts an ephemeral local **Tool Proxy Server** (HTTP/JSON-RPC or Model Context Protocol).
2. The proxy address (e.g. `http://127.0.0.1:49152`) is passed to `TestRunner.Run(ctx, scenario, fixtureEndpoint)` via environment variable (`gust_FIXTURE_ENDPOINT`) or flag.
3. When the agent issues a tool call, the proxy matches the request against `FixtureProvider.Lookup()` and returns the recorded response with injected latency or failure mode.



## 9.4 Tier 2 Cross-Language Wire Protocol

Third-party plugins in Python, TypeScript, etc., communicate via JSON-RPC 2.0 over standard input/output (`stdio`):

```json
{
  "jsonrpc": "2.0",
  "method": "evaluate",
  "params": {
    "run": { ... },
    "expected": { ... },
    "context": { ... }
  },
  "id": 1
}
```

---



# 10. The 10 MVP Deterministic Evaluators

All built-in evaluators run deterministically with zero network calls and zero cost:


| Evaluator          | Checks                                                                 | Failure Evidence Provided                        |
| ------------------ | ---------------------------------------------------------------------- | ------------------------------------------------ |
| `TaskSuccess`      | Final outcome status is completed and output matches expected criteria | Actual status, error string, output diff         |
| `ToolSelection`    | Correct tools were selected according to expected criteria             | Missing tools, extraneous tools called           |
| `ToolArguments`    | Arguments match exact values, JSON schemas, or regex patterns          | Argument diff, JSON path mismatch details        |
| `ToolSequence`     | Tools were called in required causal order / DAG                       | Out-of-order calls, broken transitions           |
| `ForbiddenTool`    | Agent did NOT call any forbidden or disallowed tools                   | Forbidden tool name, arguments, call timestamp   |
| `RequiredTool`     | Agent called all mandatory tools at least once                         | List of uncalled mandatory tools                 |
| `MaxSteps`         | Step count did not exceed defined budget                               | Total steps executed vs limit                    |
| `MaxLatency`       | Trajectory latency did not exceed threshold                            | Observed total/step duration vs limit            |
| `ErrorRecovery`    | Agent recovered after tool failure or returned error                   | Unhandled error span, failed retry sequence      |
| `SchemaValidation` | Final output and tool call arguments conform to declared JSON Schema   | Schema violation errors and offending JSON paths |


---



# 11. Fixtures & Recorded Environment



## 11.1 Stateful & Sequential Fixtures

In real scenarios, agents poll tools (e.g. `check_status() -> PENDING -> PENDING -> DONE`).
gust provides two fixture resolution modes:

1. **Canonical Hash Match (**`exact_hash`**):** Look up `SHA256(canonical_json(args))`. Ideal for idempotent queries.
2. **Sequential FIFO Match (**`ordered_sequence`**):** A queue of responses for the same tool. Each call dequeues the next response.
3. **Hybrid Match (**`prefer_exact_then_sequence`**):** Evaluates exact hash first; falls back to sequential queue.



## 11.2 Failure Injection Modes

Fixtures can simulate real-world environment failures:

- `success`: Normal 200 OK response.
- `timeout`: Server hangs until timeout threshold.
- `malformed`: Returns invalid JSON, truncated bytes, or unexpected schema.
- `slow`: Injects simulated latency (`delay_ms`).
- `partial_failure`: HTTP 429 Rate Limit or HTTP 500 Internal Error.

---



# 12. Mutation Testing Engine (Replay Mode)

The Mutation Engine operates exclusively in **Replay Mode** to guarantee 100% deterministic reproducibility:

```text
Synthetic / Golden AgentRun
           |
           v
     Mutator Injection (9 Classes)
           |
           v
      MutatedRun
           |
           v
   Deterministic Evaluator Suite
           |
           v
      Detection? (Killed vs Survived)
```



## 12.1 Nine MVP Mutation Classes

1. `remove_required_tool`: Drops a critical tool span from the trace.
2. `wrong_tool`: Replaces a tool call with an incorrect alternative.
3. `corrupt_argument`: Corrupts argument keys or values.
4. `duplicate_call`: Duplicates a tool call with identical arguments.
5. `infinite_loop`: Injects repetitive cycle of tool invocations.
6. `skip_recovery`: Deletes retry attempt following an error.
7. `excessive_tool_calls`: Spams tool calls until budget exceeded.
8. `introduce_forbidden_tool`: Injects a forbidden tool call.
9. `change_final_output`: Mutates final output string or payload.



## 12.2 Golden Metrics

- **Detection Rate:** $\ge 90$ (mutants detected by evaluators).
- **False Positive Rate:** $\le 5$ (evaluators reporting failure on unmutated `GOOD` traces).

---



# 13. Probabilistic Test Scenarios & Statistical Engine



## 13.1 Why Probabilistic Testing is Mandatory

Because LLMs are stochastic, a single run ($n=1$) produces a binary boolean that is statistically meaningless. gust models agent correctness as a binomial parameter $p$ (true success probability) and computes confidence intervals over $N$ repeated samples.

## 13.2 Wilson Score Interval Closed-Form Specification

Given $n$ independent trials with $k$ passes, the observed pass rate is:
$$\hat{p} = \frac{k}{n}$$

For confidence level $1 - \alpha$ (default 95%, $z = 1.95996$), the Wilson score interval $[p_{lower}, p_{upper}]$ is:

$$p_{center} = \frac{k + \frac{z^2}{2}}{n + z^2}$$
$$p_{margin} = \frac{z}{n + z^2} \sqrt{\frac{k(n - k)}{n} + \frac{z^2}{4}}$$
$$p_{lower} = \max\left(0.0,  p_{center} - p_{margin}\right)$$
$$p_{upper} = \min\left(1.0,  p_{center} + p_{margin}\right)$$

### Corner Cases & Boundary Handling:

- **$n < n_{min}$ (default 5):** Return `INSUFFICIENT_SAMPLES`. Never report a verdict.
- **$k = 0$ (Zero Passes):** $p_{lower} = 0.0$, $p_{upper} = \frac{z^2}{n + z^2}$. Handled cleanly without division by zero.
- **$k = n$ (All Passed):** $p_{lower} = \frac{n}{n + z^2}$, $p_{upper} = 1.0$.
- **Clamping:** Interval is strictly clamped to $[0.0, 1.0]$.



## 13.3 Verdict Classification Rules

Given a scenario's `minimum_pass_rate` $P_{min}$ (e.g. 0.95):

- `PASS`: $p_{lower} \ge P_{min}$ (Evidence proves pass rate meets threshold at stated confidence).
- `FAIL`: $p_{upper} < P_{min}$ (Evidence proves pass rate falls below threshold).
- `FLAKY`: $p_{lower} < P_{min} \le p_{upper}$ (Inconclusive; confidence interval spans across threshold).
- `INSUFFICIENT_SAMPLES`: Sample count $n < n_{min}$.

```text
Pass Rate Scale:  0.0 ------------------- P_min (0.95) ------------------- 1.0
FAIL:            [====]
FLAKY:                        [===================]
PASS:                                                [============]
```

---



# 14. Policy Engine & Gating

```yaml
policy:
  version: "1.0"
  name: "production-ci-gate"

  rules:
    # Hard constraints (zero tolerance across all samples)
    hard_constraints:
      forbidden_tools: 0
      schema_violations: 0

    # Soft constraints & reliability thresholds
    reliability:
      default_minimum_pass_rate: 0.95
      min_samples_for_verdict: 5
      on_flaky: warn # warn | fail | ignore

    # Baseline regression thresholds
    regression:
      max_pass_rate_drop: 0.02
      max_latency_increase_ratio: 0.15
```



### CI Exit Codes:

- `0`: All scenarios `PASS` (or `FLAKY` when `on_flaky: warn` or `ignore`).
- `1`: One or more scenarios `FAIL`, hard constraint violated, or regression detected.
- `2`: Configuration, schema, or runtime error.
- `3`: Flaky failure when `on_flaky: fail`.

---



# 15. Content Addressing & RFC 8785 Canonicalization

To ensure hashes (`input_hash`, `content_hash`) are bit-for-bit identical regardless of programming language or formatting:

- All JSON objects are canonicalized according to **RFC 8785 (JSON Canonicalization Scheme - JCS)**.
- Keys are sorted lexicographically by UTF-16 code units.
- Whitespace outside strings is eliminated.
- Numbers follow ECMAScript standard JSON serialization.
- Hashes are formatted as lowercase hex strings: `sha256:<64_hex_digits>`.

---



# 16. TestScenario Extraction Workflow (`scenario from-run`)

Directly enforcing the **Trace-is-not-the-test** principle:

```bash
gust scenario from-run captured_run.json --output scenarios/refund.yaml
```



### Safety Safeguards:

1. Candidate `fixtures` are generated from captured tool inputs and outputs.
2. The `assertions` block is explicitly emitted with empty placeholders and instructional comments.
3. The command **never** synthesizes assertions from what the agent actually did in the trace.
4. `provenance.reviewed_by` is left blank; CI rejects unreviewed scenarios.

---



# 17. Security & Sandboxing

1. **Tier 1 (Go Plugins):** Compiled into binary or executed as trusted internal libraries.
2. **Tier 2 (External Evaluators & Mutators):** Executed as non-root subprocesses with restricted environment variables, no incoming/outgoing network access, and strict execution timeouts.
3. **TestRunners:** The only plugin type permitted network access (to reach agent processes or LLM endpoints). Must be explicitly declared in configuration.

---



# 18. CLI Command Specification

```bash
# Mode 1: Evaluate captured run without execution
gust analyze run.json --policy policy.yaml

# Mode 2: Deterministic replay against recorded fixtures (no live LLM)
gust replay run.json --fixtures fixtures/

# Mode 3: Probabilistic testing of real agent with real LLM
gust test scenario.yaml --samples 20 --concurrency 4

# Mutation benchmark in replay mode
gust mutate run.json --classes all --n 100

# Regression comparison between baseline and candidate experiments
gust compare baseline_exp.json candidate_exp.json --significance 0.95

# Assisted scenario extraction from production trace
gust scenario from-run incident_run.json --output scenario.yaml
```

---



# 19. Phased Roadmap


| Phase   | Milestone                       | Focus Areas                                 | Key Deliverables & Exit Criteria                                             |
| ------- | ------------------------------- | ------------------------------------------- | ---------------------------------------------------------------------------- |
| **0**   | Foundation & ADRs               | Spec, architecture records, license         | Complete architecture decisions, Apache-2.0 license recorded.                |
| **1**   | Core Schemas & JCS              | Schemas, domain models, RFC 8785 JCS        | Round-trip tests pass in Go and Python; canonical hashes match.              |
| **2**   | Interfaces & Wire Protocol      | Go contracts, JSON-RPC stdio protocol       | Extensible plugin system; test runners and evaluators pluggable.             |
| **3**   | Deterministic Evaluators        | 10 built-in evaluators                      | Pass evaluation benchmark; single-core throughput $\ge 1,000$ cases/sec.     |
| **4**   | Analyze, Replay & Fixture Store | Fixture engine, stateful queues, mock proxy | 100 repeated Replay runs produce identical results; zero live network calls. |
| **5**   | Mutation Engine                 | 9 mutation classes                          | Detection Rate $\ge 90$, FPR $\le 5$ on golden corpus.                       |
| **6**   | Probabilistic Test Mode         | Real agent execution, Wilson score stats    | Correctly classifies 99% agent as `PASS` and 85% agent as `FLAKY`.           |
| **7**   | Policy Engine                   | Rules, hard/soft constraints, gating        | Policy evaluates `PASS`/`FAIL`/`FLAKY`; `on_flaky` modes verified.           |
| **8**   | Scenario Extraction             | `scenario from-run` workflow                | Structural safeguard: assertions never auto-populated.                       |
| **9**   | CLI, CI & Demo Verification     | Cobra CLI, exit codes, golden demo          | Killer demo runs end-to-end against Ollama/mock at $0 cost.                  |
| **10+** | OTel, Multi-Agent, Ecosystem    | OpenInference, clustering, registry         | External ingestion, PII redaction, community plugins.                        |


---



# 20. Definition of Done (MVP)

- [ ] All 8 JSON Schemas formally defined (Draft 2020-12) and validated.
- [ ] Canonical serialization implemented per RFC 8785 JCS.
- [ ] Tier 1 and Tier 2 plugin interfaces implemented and tested.
- [ ] All 10 deterministic evaluators implemented with structured evidence.
- [ ] Fixture Store with hash-based and stateful sequential resolution.
- [ ] Ephemeral Tool Proxy Server for live agent tool interception.
- [ ] Mutation Engine with 9 classes achieving $\ge 90$ detection and $\le 5$ FPR.
- [ ] Mode 3 Test Runner executing parallel samples with local LLM (Ollama).
- [ ] Wilson score statistical engine correctly classifying `PASS`, `FAIL`, `FLAKY`, and `INSUFFICIENT_SAMPLES`.
- [ ] Policy Engine supporting hard constraints, soft thresholds, and `on_flaky` behavior.
- [ ] Scenario extraction CLI command safeguarding against bug enshrinement.
- [ ] Single static Go binary with subcommands: `analyze`, `replay`, `test`, `mutate`, `compare`, `scenario`.
- [ ] All tests run offline with zero API cost.

---



# 21. Changelog: v0.4 → v0.5


| #   | Area                              | Change Description                                                                              | Rationale                                                                                           |
| --- | --------------------------------- | ----------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| 1   | Fixture Engine (§11.1, §15.1)     | Added stateful FIFO sequence queues alongside hash-based lookup (`match_strategy`).             | Resolves infinite loops during tool polling (e.g. `check_status()`).                                |
| 2   | Tool Interception Protocol (§9.3) | Added ephemeral Mock Tool Proxy (HTTP/JSON-RPC/MCP).                                            | Enables live agents in any language to query fixtures during Test mode.                             |
| 3   | Statistical Rigor (§13.2)         | Formalized closed-form Wilson Score interval with boundary clamping and zero-variance handling. | Guarantees exact, reproducible statistical intervals across all implementations.                    |
| 4   | Content Addressing (§15, §20)     | Mandated RFC 8785 JSON Canonicalization Scheme (JCS) with SHA-256.                              | Eliminates cross-language hash mismatch caused by key ordering or whitespace.                       |
| 5   | Mutator Lifecycle (§9.2)          | Introduced `MutationOutcome` with `Mutated`, `Skipped`, and `Error` statuses.                   | Prevents unapplicable mutations from corrupting Detection Rate and FPR metrics.                     |
| 6   | Assertion Criticality (§7.4, §14) | Added `hard` vs `soft` criticality to assertions and policy rules.                              | Enforces zero-tolerance safety policies while allowing probabilistic rates for non-fatal behaviors. |
| 7   | Go Idioms & Thread Safety (§9.1)  | Standardized `context.Context`, streaming models, and value semantics on core interfaces.       | Ensures cancellation, timeouts, and concurrency safety during parallel sampling.                    |


