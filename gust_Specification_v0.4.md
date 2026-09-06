# gust: Test Infrastructure for Autonomous Software
## Design, Requirements, Phased Roadmap, and Verification Plan

**Status:** v0.4 (supersedes v0.3)
**Project type:** Open-source infrastructure/library
**Reference implementation language:** Go, from Phase 0 (unchanged since v0.2)
**License:** Apache-2.0
**Audience for this document:** intended to be handed directly to an implementation team/coding agent for planning and build-out. Where a section says "MVP," treat it as literally in scope for the first buildable milestone; everything else is sequenced but deferred.

> **This revision is a second thesis refinement, not a patch.** v0.3 correctly elevated Replay + Mutate + Policy over "yet another evaluator catalog," but it collapsed two genuinely different things under the word "replay": (a) deterministically re-serving recorded tool responses so a **live agent with a live LLM** can be tested against a fixed world, and (b) mechanically reconstructing a **past execution** for analysis, with no live decision-making at all. Conflating these hides the central engineering problem of the whole project: **LLMs are not deterministic, so "does this agent pass this test" is not a boolean — it's a rate, measured over repeated samples.** v0.4 makes this explicit with three named execution modes (§5) and moves probabilistic, repeated-sample testing into the **MVP**, not a later phase — because `assert x == true` is simply the wrong primitive for a system whose core component is stochastic. Full diff in §40.

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

1. **A trace is not the test.** A captured `AgentRun` is evidence — it tells you what happened once. A **`TestScenario`** (input + a controlled environment + independent assertions about correct behavior) is the actual test artifact. Building a fancy trace replayer instead of this is the single biggest way this project could go wrong (§4.1).
2. **Three distinct execution modes exist, and the spec must never blur them: Analyze, Replay, and Test (§5).** Only one of them — Test — invokes a real agent with a real LLM, and it is the only one that tells you whether *today's* agent is still correct.
3. **Because the LLM is not deterministic, a single Test-mode execution proves very little. Probabilistic, repeated-sample assertions (§17) are therefore MVP scope, not a future enhancement** — a test suite that reports `PASS`/`FAIL` from one sample when the true pass rate is 85% is not testing infrastructure, it's a coin flip with extra steps.

Mutation testing (§16) remains the project's flagship trust metric for the deterministic parts of the stack (evaluators, policy aggregation) — it operates in Replay mode, against synthetic or recorded agents, and does not require the probabilistic machinery in §17, which exists specifically for Test-mode's live-LLM nondeterminism.

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

A traditional unit test that behaved this way would be considered broken. An agent test that behaves this way is normal, and pretending otherwise (by running it once and trusting the result) is the single most common way agent test suites lie to their users. gust's job is to measure and report the *rate*, honestly, not to paper over it with a single boolean (§17).

## 2.3 Replaying old mistakes is not testing

If a production trace recorded `cancel_order(122)` (the wrong order), and a "regression test" simply replays that exact call and checks it still happens, it has enshrined the bug as expected behavior. The correct regression test asks the **new agent to independently decide**, given the same *environment*, and checks the *independently made* decision against the *correct* expected behavior — not against what happened before (§4.1, §17.1).

---

# 3. Competitive Landscape & Real Differentiation

Named competitors and their real capabilities, from v0.3, still hold and are not repeated in full here (see §37 for the citation list). Updated conclusion:

| Project | Strong at | Still doesn't make first-class |
|---|---|---|
| DeepEval | Trajectory/component agent metrics, tracing, CI/CD | Mutation-validating its own evaluators; a formal recorded-environment protocol; probabilistic pass-rate assertions as a primary testing primitive rather than a single run. |
| Braintrust | Datasets, immutable experiments, CI regression, production feedback loops; recommends stubbing dependencies | The stubbing recommendation is a *pattern*, not a schema-backed `TestScenario`/`Environment` protocol with mutation validation and reliability sampling built in. |
| Arize Phoenix | Deterministic + LLM evaluators, OTel-native, fast batch evaluation | Same gap: no first-class distinction between "replaying a recorded trace" and "testing a live agent against a controlled environment," and no built-in reliability-under-repeated-sampling metric. |

### Undefended territory (updated — six points, was five in v0.3)

1. Mutation-testing the evaluator suite itself (Detection Rate + False Positive Rate), unchanged from v0.3.
2. A formal, versioned recorded-fixture/replay protocol.
3. A policy engine as the actual pass/fail authority, decoupled from any evaluator.
4. A cross-language wire protocol (§13).
5. Benchmarked deterministic-evaluation throughput.
6. **A first-class, explicit separation between "analyzing a past execution," "deterministically replaying a recorded execution," and "testing a live agent's independent decision-making," with the third treated as inherently probabilistic and measured with repeated sampling and confidence intervals, not a single assert.** This is the sharpest, newest differentiator, and it is a direct consequence of taking LLM nondeterminism seriously as a testing problem rather than an inconvenience to be ignored.

---

# 4. Product Vision & Core Principle

## 4.1 The core principle (new, load-bearing)

> **A trace is not the test. A trace is evidence from which a reproducible test scenario can be created.**

This governs every design decision downstream:

- `AgentRun` (a captured execution) and `TestScenario` (a reusable test artifact, §17) are **different objects with different schemas** — one is a record, the other is a specification.
- Converting a production `AgentRun` into a `TestScenario` is a deliberate, assisted-but-human-reviewed transformation (§17.4), not an automatic "replay this exact sequence" operation.
- A `TestScenario`'s environment can and should be varied from what was recorded (e.g., two orders instead of one) precisely because the goal is to test the agent's *general* decision-making in that class of situation, not to reproduce one specific trace byte-for-byte.

## 4.2 Original core principle (unchanged, still enforced by the wire protocol, §13)

Any agent should be able to produce an gust-compatible execution, regardless of model provider, agent framework, programming language, deployment environment, tool provider, or tracing backend.

## 4.3 Long-term architecture

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
                 engine bench)   probabilistic replayed     including
                                 sampling,     agents,       PASS /
                                 §17)          measure       FAIL /
                                               Detect+FPR)   FLAKY)
        |            |            |            |            |
        +------------+------------+------------+------------+
                                    |
                                    v
                          Regression Result
                     (structured evidence, §20;
                      pass rate + confidence
                      interval where probabilistic)
                                    |
                     +--------------+--------------+
                     |                             |
                    CI                        Production
```

---

# 5. The Three Execution Modes (new, central section)

This section is the load-bearing addition of v0.4. Every phase, schema, and CLI command downstream is organized around these three modes, and the spec must never blur them.

## 5.1 Mode 1 — Analyze

```text
Existing AgentRun -> Evaluators -> "What happened?"
```

- **No execution of any kind.** The agent is not invoked. The LLM is not invoked. This mode consumes an already-captured `AgentRun` and runs the evaluator suite (§14) and policy engine (§18) against it.
- **Purpose:** understand a specific past execution, debug an evaluator, inspect a production incident, generate a report.
- **Determinism:** total — the same `AgentRun` + evaluator versions always produce the same result.
- **CLI:** `gust analyze <run.json>`

## 5.2 Mode 2 — Replay

```text
Existing AgentRun -> Recorded environment (Fixtures) -> Deterministic re-execution -> New AgentRun
```

- The agent's **control flow** is re-executed, but against **recorded Fixtures** (§15) rather than live services, and — critically — **without necessarily invoking a live LLM**: where the original trace recorded an LLM decision, Replay mode can feed that recorded decision back in directly, or invoke a `MockLLM`/deterministic stub, rather than calling a real model.
- **Purpose:** deterministic reproduction of a scenario, evaluator development against a stable input, mutation testing (§16 — the Mutation Engine runs entirely in this mode, against synthetic or recorded agents, precisely because it needs bit-for-bit reproducibility to make Detection Rate/False Positive Rate meaningful), and engine performance benchmarking (§24).
- **Determinism:** total, by construction — this is what makes §16's mutation benchmark and §24's non-functional targets measurable at all.
- **CLI:** `gust replay <run.json>`

## 5.3 Mode 3 — Test *(the regression-testing primitive; MVP; inherently probabilistic)*

```text
TestScenario (input + Environment + Assertions)
       |
       v
  REAL Agent -> REAL LLM -> controlled Environment (Fixtures/mocks)
       |
       v
   New AgentRun  (repeated N times, §17)
       |
       v
   Evaluators -> Policy -> PASS / FAIL / FLAKY, with a measured pass rate and confidence interval
```

- **This is the only mode that tells you whether today's agent is still correct.** The agent and the LLM are both real and live; only the *environment* (tool/API/DB responses) is controlled, via the same Fixture mechanism as Replay mode.
- **The agent must make its own decision.** A `TestScenario`'s assertions describe *correct* behavior for the given environment — they are authored independently of whatever a recorded trace happened to do, specifically so that replaying an old mistake can never look like a passing test (§4.1, §17.1).
- **Because the LLM is not deterministic, a single execution is not a meaningful result.** Test mode runs a scenario **N times** (configurable, default in §17.3) and reports a pass rate with a statistical interval, not a bare boolean, unless the scenario is explicitly marked as expected to be fully deterministic (rare — e.g., a scenario whose assertions only touch deterministic pre-LLM logic).
- **CLI:** `gust test <scenario.yaml>` (see §21 for flags, §17 for the full statistical treatment)

## 5.4 Why all three must exist, and why none can substitute for another

- Analyze alone would make gust "a fancy trace viewer" — useful, but not testing infrastructure.
- Replay alone would make mutation testing possible but would never tell you whether the *current* agent is correct, since it never invokes a live decision-maker.
- Test alone, without Analyze/Replay, would have no fast, deterministic, zero-cost validation path — every CI run would require live LLM calls, which is slow, costly, and reintroduces exactly the flakiness Replay-mode mutation testing is designed to avoid measuring against.

The combination is deliberate: **Replay + Mutate validate that the evaluation and policy machinery itself is trustworthy, cheaply and deterministically; Test then applies that trusted machinery to the one thing that actually needs a live LLM — checking whether today's agent still behaves correctly, honestly reported as a rate.**

---

# 6. Goals

## P0 goals (MVP)

1. Core execution model: `AgentRun`, `Span`, `Fixture`, `TestScenario`, `Policy` (§9, §17).
2. Extensible evaluator/mutator/fixture-provider interface — Tier 1 (native Go) and Tier 2 (wire protocol), §13.
3. Deterministic evaluation only — no LLM-based evaluator/judge in the MVP (§7).
4. Evaluate complete trajectories, not just final outputs.
5. **Mode 1 (Analyze) and Mode 2 (Replay) fully working, including the Fixture Store and failure-injection modes (§15).**
6. **Mode 3 (Test) fully working, including a real-agent-runner interface, `TestScenario` schema, and — non-negotiable for MVP — probabilistic, repeated-sample assertions with a statistical pass-rate interval (§17). This is the single most important addition of v0.4 and is explicitly MVP, not deferred.**
7. **A working Mutation Engine (Replay-mode) measuring Detection Rate + False Positive Rate (§16) — still the primary trust metric for the deterministic stack.**
8. A Policy Engine that aggregates evaluator output — including probabilistic pass-rate assertions — into `PASS`/`FAIL`/`FLAKY` verdicts with structured evidence (§18, §19).
9. **A `TestScenario`-from-`AgentRun` extraction workflow** (assisted, human-reviewed) so the production-failure -> regression-test loop described in §2.3/§4.1 is real tooling, not aspiration, even if it's a manual/semi-automated CLI helper in the MVP (§17.4).
10. Reproducible, content-addressed datasets, fixture sets, and test scenarios.
11. Regression comparison with statistical significance from the first release — now understood as two related but distinct statistical questions (§20): "is this one scenario reliable?" (§17) and "did the aggregate pass rate regress between baseline and candidate?" (§20).
12. CLI and CI integration, distributed as a single static binary.
13. Fully offline operation for everything except live Test-mode runs against a real LLM (§22) — Analyze, Replay, and Mutate must work with zero network access; Test mode against a local model (Ollama) must also work with zero cost.
14. A minimal security/trust model for third-party evaluators/mutators before any plugin ecosystem opens.

## P1 goals

1. OpenTelemetry/OpenInference ingestion adapter feeding Analyze mode.
2. Framework adapters, including at least one non-Python/non-Go-only language.
3. Optional LLM-based evaluation (a judge), added only after the deterministic + probabilistic Test-mode stack is proven.
4. **Automated production trace ingestion feeding the `TestScenario` extraction workflow (§17.4) at scale**, PII-safe by default — the MVP already has the manual/assisted version (P0.9); this phase automates detection of "interesting" traces and proposes extractions.
5. Failure mining / clustering to reduce duplicate proposed scenarios.
6. Statistical Regression Engine v2 (bootstrap, effect size, minimum sample size guidance) — upgrading both the §17 single-scenario reliability gate and the §20 baseline-vs-candidate gate.
7. Multi-agent evaluation.
8. Human annotation/calibration gating any future judge provider's promotion to "stable."

## P2 goals

1. AEE public leaderboard (§25), benchmarked empirically against DeepEval/Phoenix/etc.
2. Community-contributed evaluators/mutators, sandboxed and checksummed.
3. Cross-framework benchmark results.
4. Standard semantic conventions contribution back to OpenTelemetry GenAI conventions.
5. Evaluation/scenario registry.
6. Production continuous evaluation.
7. Safety/security evaluator ecosystem.

---

# 7. Non-Goals

The MVP must not attempt to:

- build an agent runtime;
- build an LLM;
- compete with model providers;
- replace or duplicate OpenTelemetry/OpenInference — consume their traces via Analyze mode instead;
- build a large hosted dashboard;
- provide hundreds of metrics;
- claim a single universal "intelligence score";
- require paid AI APIs — Test mode must work against a local model for $0 (§23);
- require a cloud account;
- include an LLM-as-judge evaluator at all (unchanged from v0.3 — still explicitly deferred, §6);
- become a multi-language monorepo before the schema is stable;
- **treat a single Test-mode execution as a meaningful pass/fail result.** A scenario that has not been run at least the configured minimum sample count (§17.3) must not be reported as a bare `PASS`/`FAIL` — it must be reported as `INSUFFICIENT_SAMPLES` or similar, so the tool never accidentally implies a confidence it doesn't have;
- **automatically promote a replayed trace into a "passing test" without an independently authored assertion** — this is precisely the "replaying the old mistake" failure mode from §2.3/§4.1, and the tooling must make it structurally awkward to do accidentally (e.g., `gust scenario from-run` requires the human to fill in `assertions`, it does not default them to "whatever happened").

---

# 8. Design Principles

## 8.1 Trace-is-not-the-test (new, primary — §4.1)
A captured `AgentRun` is evidence, not a test. `TestScenario` is the test artifact. Extraction from one to the other is deliberate and human-reviewed.

## 8.2 Probabilistic honesty (new, primary — §5.3, §17)
Any assertion about a live, LLM-driven execution's correctness must be expressed and reported as a rate with a stated sample size and confidence treatment, unless the scenario is provably deterministic end-to-end. A tool that reports `PASS` from `n=1` on a stochastic system is misleading its users, even if that single run happened to pass.

## 8.3 Mode separation (new, primary — §5.4)
Analyze, Replay, and Test are architecturally distinct code paths with distinct guarantees. Code must not, for convenience, let a Replay-mode call silently invoke a live LLM, or let a Test-mode run silently skip sampling because "it usually works."

## 8.4 Offline-first
The core must work without OpenAI, Anthropic, Gemini, paid inference, or internet access for Analyze, Replay, and Mutate. Test mode must work against a $0 local model.

## 8.5 Deterministic-first
Prefer deterministic checks over LLM judges wherever the property in question can be checked deterministically (unchanged from v0.3 §7.2).

## 8.6 Mutation-validated evaluation
The evaluator suite's own reliability (Detection Rate + False Positive Rate) is measured via Replay-mode mutation testing and published, not assumed (unchanged from v0.3 §7.4).

## 8.7 Policy decides, evaluators measure
A Policy Engine, not any individual evaluator, issues the final verdict, now including `FLAKY` as a first-class outcome alongside `PASS`/`FAIL` (updated from v0.3 §7.5 to account for §17).

## 8.8 Evidence over scores
`EvaluationResult` carries structured evidence sufficient to explain a failure without re-running it (unchanged from v0.3 §7.6).

## 8.9 Model-based evaluation is a plugin, deferred
Unchanged from v0.3 §7.7 — an LLM judge, when added in P1, is a Tier 1/Tier 2 evaluator like any other, never hard-coded.

## 8.10 OpenTelemetry/OpenInference compatible, not competing
Unchanged from v0.3 §7.9.

## 8.11 Extensible by design — two tiers
Unchanged from v0.3 §7.10 (Tier 1 native Go, Tier 2 wire protocol), extended in §13 to cover `TestRunner` (the interface that invokes a real agent in Test mode) alongside evaluators, mutators, and fixture providers.

## 8.12 Stable core, experimental extensions
Unchanged from v0.3 §7.11.

## 8.13 Security & trust model
Unchanged from v0.3 §7.12, extended in §27 to cover `TestRunner` plugins (a live-agent invocation is a stronger trust boundary than a pure evaluator — see §27).

## 8.14 Interface versioning policy
Unchanged from v0.3 §7.13.

---

# 9. Core Domain Model

Defined first as **JSON Schema** in `spec/schemas/` (source of truth); Go structs are generated from or validated against it.

```text
AgentRun
Trace
Span
ToolCall
Fixture / RecordedResponse           (provenance: "recorded" | "authored")
TestScenario                         (new — the actual test artifact, §17)
Assertion                            (new — deterministic OR probabilistic, §17.2)
Task
Dataset
DatasetCase                          (now a thin wrapper around TestScenario, §21)
Evaluator
EvaluationResult (with structured Evidence, §19)
Mutator
MutationResult
Policy                               (extended with probabilistic clauses, §18)
Verdict                              (PASS | FAIL | FLAKY | INSUFFICIENT_SAMPLES)
Experiment
Comparison
Regression
ReliabilityResult                    (new — pass rate + interval for one scenario, §17.3)
```

## AgentRun (unchanged from v0.3)

```json
{
  "schema_version": "0.1",
  "run_id": "run_123",
  "agent": { "name": "support-agent", "version": "1.2.0" },
  "task": { "id": "refund-001", "input": "Refund order 123" },
  "trace": [],
  "outcome": {},
  "metadata": {}
}
```

## Fixture (extended — `provenance` field is new)

```json
{
  "fixture_id": "fx_001",
  "tool": "get_orders",
  "input_hash": "sha256:...",
  "recorded_input": { "customer_id": 42 },
  "recorded_response": { "status": "success", "body": [{"id": 122, "status": "DELIVERED"}, {"id": 123, "status": "PROCESSING"}] },
  "recorded_at": "2026-06-01T12:00:00Z",
  "mode": "success",
  "provenance": "recorded"
}
```

`provenance: "recorded"` means captured from a real production/staging execution (§15.3); `provenance: "authored"` means hand-written directly for a synthetic `TestScenario` — both are valid, and a scenario extracted from production (§17.4) typically starts as `recorded` fixtures that a human may then edit into `authored` variants (e.g., "two orders instead of one," per §2's worked example).

## TestScenario (new — the actual test artifact)

```yaml
id: cancel_latest_order
description: >
  Given two orders where the most recent is still processing, the agent
  must cancel the correct (most recent) one, not the older delivered one.

input: "Cancel my latest order"

environment:
  fixtures:
    - fixture: get_orders
      response:
        - { id: 122, status: DELIVERED }
        - { id: 123, status: PROCESSING }

assertions:
  - type: tool_call
    tool: cancel_order
    arguments:
      order_id: 123

  - type: forbidden_tool_call
    tool: cancel_order
    arguments:
      order_id: 122

reliability:
  samples: 20
  minimum_pass_rate: 0.95
  confidence: 0.95

provenance:
  source: production_trace
  source_run_id: run_18291
  extracted_at: "2026-06-01T13:00:00Z"
  reviewed_by: "jane@example.com"
```

Key properties, directly enforcing §4.1/§8.1:

- `assertions` describe **correct** behavior, authored independently — they are never auto-populated from "what the source trace did."
- `environment` is a set of Fixtures (§9's `Fixture`), which may be edited/varied from what was originally recorded.
- `reliability` is present on every `TestScenario` by default (§17.2) — a scenario without a `reliability` block is treated as `samples: 1` **only** if explicitly marked `deterministic: true`, which must be justified (e.g., a scenario that only exercises pre-LLM deterministic routing logic).
- `provenance` records where the scenario came from, supporting the "why does this test exist" question a maintainer will eventually ask.

## Assertion (new)

```json
{
  "type": "tool_call | forbidden_tool_call | required_tool | schema_valid | max_steps | max_latency_ms | custom",
  "tool": "cancel_order",
  "arguments": { "order_id": 123 },
  "evaluator": "tool_correctness"
}
```

Each `Assertion` type maps to one of the MVP evaluators (§14) under the hood — the `TestScenario` format is a human-authorable surface over the same evaluator machinery Analyze/Replay mode already uses.

---

# 10. Span Types

Unchanged from v0.3 §11: `agent`, `llm`, `tool`, `retrieval`, `memory`, `plan`, `error` (MVP); `agent_handoff`, `workflow`, `human_intervention`, `guardrail`, `sandbox`, `browser`, `code_execution`, `evaluation` (future).

---

# 11. High-Level Architecture

```text
+------------------------------------------------------------------+
|                       Agent Application                          |
|              (Go / Python / TypeScript / anything)               |
+-------------------------------+----------------------------------+
                                |  instrumentation, or OTel/OpenInference ingestion
                                v
+------------------------------------------------------------------+
|                     gust SDK (per language)                 |
+-------------------------------+----------------------------------+
                                |  canonical AgentRun (JSON)
                                v
+------------------------------------------------------------------+
|             gust Core Engine  (Go, single static binary)    |
|                                                                    |
|  +---------+  +---------+  +---------+  +---------+  +---------+  |
|  | Analyze |  | Replay  |  |  Test   |  | Mutate  |  | Policy  |  |
|  | (§5.1)  |  | (§5.2)  |  | (§5.3)  |  | (§16)   |  | (§18)   |  |
|  |no exec  |  |determ., |  |REAL     |  |synthetic|  |PASS/    |  |
|  |         |  |no live  |  |agent +  |  |or       |  |FAIL/    |  |
|  |         |  |LLM      |  |REAL LLM,|  |replayed |  |FLAKY,   |  |
|  |         |  |         |  |sampled  |  |agents,  |  |evidence |  |
|  |         |  |         |  |N times  |  |Detect+  |  |         |  |
|  |         |  |         |  |(§17)    |  |FPR      |  |         |  |
|  +---------+  +---------+  +---------+  +---------+  +---------+  |
+---------------+--------------------------------+------------------+
                |                                  |
                v                                  v
      Tier 1: native Go evaluators/       Tier 2: Wire Protocol
      mutators/fixture providers/         JSON-over-stdio / gRPC
      TestRunners
                           |
                           v
              Evaluation Result + ReliabilityResult (§19, §17.3)
                           |
             +-------------+-------------+-------------+
             |             |             |              |
            CLI           CI       OTel Exporter   Public benchmark
       (Cobra, Go)   (policy-gated,  (native OTel      / AEE leaderboard
                       reliability-   Go SDK)           (P2, §25)
                       and-
                       significance
                       tested, §20)
```

---

# 12. Repository Architecture

```text
gust/
|
+-- spec/
|   +-- schemas/                 # AgentRun, Span, Fixture, TestScenario, Assertion,
|   |                             # Policy, EvaluationResult, ReliabilityResult, MutationResult
|   +-- semantic-conventions/
|   +-- evaluator-protocol/      # Tier 2 wire protocol
|   +-- versioning.md
|   +-- compatibility.md
|
+-- core/
|   +-- run/
|   +-- trace/
|   +-- analyze/                 # Mode 1 (§5.1)
|   +-- replay/                  # Mode 2: Fixture Store, deterministic re-execution (§5.2, §15)
|   +-- test/                    # Mode 3: TestRunner, sampling, reliability stats (§5.3, §17)
|   +-- evaluation/
|   +-- mutate/                  # Mutation Engine (§16)
|   +-- policy/                  # Policy Engine incl. probabilistic clauses (§18)
|   +-- scenario/                # TestScenario extraction workflow (§17.4)
|   +-- dataset/
|   +-- experiment/
|   +-- result/
|   +-- errors/
|
+-- evaluators/                  # Tier 1, built-in (§14)
+-- mutators/                    # Tier 1, built-in (§16)
|
+-- providers/
|   +-- fixture/                 # recording/replay backends (§15)
|   +-- testrunner/              # invokes the real agent + real LLM in Test mode
|   +-- judge/                   # reserved; no implementation until P1
|   +-- storage/
|   +-- exporter/
|
+-- cli/                         # analyze, replay, test, mutate, compare, scenario
|
+-- sdk/
|   +-- python/
|   +-- typescript/              # P1
|
+-- ingest/
|   +-- otel/
|
+-- contrib/
|   +-- adapters/
|   +-- evaluators/
|
+-- experimental/
|
+-- benchmarks/
|   +-- synthetic/                # for Replay-mode mutation testing (no live LLM)
|   +-- mutation/
|   +-- golden/
|   +-- calibration/              # reserved, P1
|
+-- examples/
|   +-- scenarios/                # example TestScenario files
+-- tests/
|   +-- unit/
|   +-- integration/
|   +-- contract/
|   +-- benchmark/
|   +-- mutation/
|   +-- replay/
|   +-- reliability/              # probabilistic-assertion correctness tests, §17.5
|
+-- docs/
+-- adr/
+-- CONTRIBUTING.md
+-- LICENSE
+-- README.md
```

---

# 13. Evaluator, Mutator, Fixture-Provider & TestRunner Architecture (Wire Protocol)

Native Go interfaces (Tier 1):

```go
type Evaluator interface {
    Name() string
    Version() string
    Evaluate(run AgentRun, expected *ExpectedBehavior, ctx EvaluationContext) (EvaluationResult, error)
}

type Mutator interface {
    Name() string
    Mutate(run AgentRun) (MutatedRun, error)
}

type FixtureProvider interface {
    Lookup(call ToolCall) (RecordedResponse, bool, error)
    Mode(call ToolCall) FailureMode
}

// New in v0.4 — the interface that invokes a REAL agent in Test mode.
type TestRunner interface {
    Name() string
    Run(scenario TestScenario, env FixtureProvider) (AgentRun, error)
}
```

Cross-language contract (Tier 2), unchanged in shape from v0.3, extended with a `"test_runner"` kind:

```text
{
  "protocol_version": "1.0",
  "kind": "evaluator | mutator | fixture_provider | test_runner",
  "name": "...",
  "version": "...",
  ...
}
```

`TestRunner` is the one Tier 2 plugin kind that is explicitly allowed, and expected, to make real network calls (to the LLM provider and, in staging, to the agent process) — so its sandboxing profile in §27 is deliberately different from the other three, which are expected to be side-effect-free.

A reference Tier 2 implementation ships in at least two languages (Go, Python) before Phase 2 (§30) is complete.

---

# 14. MVP Evaluators (deterministic only)

Unchanged from v0.3 §13: `TaskSuccess`, `ToolSelection`, `ToolArguments`, `ToolSequence`, `ForbiddenTool`, `RequiredTool`, `MaxSteps`, `MaxLatency`, `ErrorRecovery`, `SchemaValidation`. Each maps to one or more `Assertion` types (§9) so `TestScenario` authors can invoke them declaratively. `ResponseQuality` (LLM-based) remains explicitly out of the MVP (§7).

---

# 15. Fixtures & the Recorded Environment (Replay & Test modes)

## 15.1 Purpose

Fixtures are the shared mechanism behind both Mode 2 (Replay, no live LLM, used for mutation testing and engine debugging) and Mode 3 (Test, live agent + live LLM, used for regression testing). The difference between the modes is **whether the agent's decision-making is live**, not whether the environment is controlled — the environment is controlled in both.

```text
production:
Agent -> DB, API, Search, Payment (live)

replay or test:
Agent -> gust Fixture Store -> recorded/authored responses
```

## 15.2 Failure-injection modes (unchanged from v0.3 §14.2)

`success`, `timeout`, `malformed`, `slow`, `partial_failure` — used by both Replay-mode mutation testing and Test-mode `ErrorRecovery` assertions.

## 15.3 Recording workflow

```text
production or staging run
       |
       v
   capture (SDK instrumentation or OTel ingestion)
       |
       v
   Fixture (provenance: "recorded")
       |
       v
   Fixture Store (content-addressed, §21)
```

Fixtures may subsequently be hand-edited into `provenance: "authored"` variants when a `TestScenario` deliberately varies the environment from what was recorded (§9's worked example: two orders instead of one).

## 15.4 Verification

Fixture round-trip tests, a determinism test for Replay mode (100 identical repeated runs -> identical results), and one failure-injection test per mode in §15.2.

## 15.5 Exit criteria

Zero live network calls during Analyze, Replay, or Mutate. Test mode's *only* live calls are to the agent/LLM under test — never to the tool/API/DB layer, which is always served by Fixtures.

---

# 16. Mutation Testing Engine (Replay mode — the deterministic trust metric)

Unchanged in substance from v0.3 §16, with one clarification: **the Mutation Engine always operates in Replay mode**, against synthetic Go-emitted agents or fully-recorded traces — never against a live LLM — precisely because Detection Rate and False Positive Rate must be measured against a fixed, reproducible ground truth. It answers "is our evaluation/policy machinery trustworthy," which is a prerequisite for Test mode's probabilistic claims (§17) to mean anything at all: if the evaluators can't reliably detect a `wrong_tool` mutation in a deterministic replay, a fuzzy pass-rate number from a live, noisy LLM run is not trustworthy either.

- **Mutation classes (MVP):** `remove_required_tool`, `wrong_tool`, `corrupt_argument`, `duplicate_call`, `infinite_loop`, `skip_recovery`, `excessive_tool_calls`, `introduce_forbidden_tool`, `change_final_output`.
- **Metrics, always reported as a pair:** Detection Rate ≥ 90%, False Positive Rate ≤ 5%, on the `GOOD` class, reproduced across repeated runs.
- **CLI:** `gust mutate <agent.json> --classes all --n 100`

---

# 17. Test Scenarios & Probabilistic Assertions (new — MVP, central to v0.4)

## 17.1 Why a single execution cannot be the unit of truth

An LLM-driven agent's correctness on a given scenario is better modeled as a **latent success probability** than a fixed boolean. A single run samples from that distribution; it does not reveal it. Reporting `PASS` after one lucky sample, or `FAIL` after one unlucky one, actively misleads whoever reads the CI result. This is why §7's non-goals explicitly forbid treating `n=1` as meaningful.

## 17.2 `TestScenario` reliability block

```yaml
reliability:
  samples: 20            # number of independent Test-mode executions
  minimum_pass_rate: 0.95
  confidence: 0.95        # confidence level for the statistical test
```

Defaults (used when a scenario omits the block, unless `deterministic: true` is set): `samples: 10`, `minimum_pass_rate: 0.90`, `confidence: 0.95`. These are placeholders for the team to tune, but must exist and be documented, not silently assumed as 1.

## 17.3 Statistical treatment — `ReliabilityResult`

```text
1. Run the scenario N times through the TestRunner (§13), independently.
2. For each run, evaluate + apply per-run assertions -> per-run PASS/FAIL.
3. observed_pass_rate = passes / N
4. Compute a Wilson score confidence interval around observed_pass_rate at the
   configured confidence level.
5. Verdict:
     - PASS               if the interval's lower bound >= minimum_pass_rate
     - FAIL                if the interval's upper bound <  minimum_pass_rate
     - FLAKY               if the interval straddles minimum_pass_rate
                            (i.e., neither PASS nor FAIL condition holds —
                            evidence is inconclusive at this sample size)
     - INSUFFICIENT_SAMPLES if N is below a configured minimum (default 5),
                            regardless of the observed rate
```

```json
{
  "scenario_id": "cancel_latest_order",
  "samples": 20,
  "passes": 17,
  "observed_pass_rate": 0.85,
  "confidence_interval": [0.64, 0.95],
  "minimum_pass_rate": 0.95,
  "verdict": "FLAKY",
  "per_run_evidence": [ "...EvaluationResult per run, §19..." ]
}
```

`FLAKY` is a genuinely new, first-class outcome (§8.7) — it is not an error state, it is an honest statement that more samples are needed to distinguish "this agent is reliable" from "this agent is broken," and it must render distinctly from `PASS`/`FAIL` everywhere (CLI, CI annotations, JSON output).

## 17.4 `TestScenario`-from-`AgentRun` extraction workflow (MVP, assisted, human-reviewed)

Directly implements §2.3/§4.1's "trace is not the test" principle as real tooling:

```bash
gust scenario from-run run_18291.json --output cancel_latest_order.yaml
```

Behavior:

1. Reads the `AgentRun`, extracts its `input` and the tool calls' recorded inputs/outputs as candidate `Fixture`s (`provenance: "recorded"`).
2. Emits a `TestScenario` skeleton with the `environment` populated from those fixtures.
3. **Leaves `assertions` empty (or filled with `# TODO: assert correct behavior, not what happened`) — it never defaults assertions to "whatever the source run did."** This is the structural safeguard against silently re-encoding a bug as a passing test (§7's non-goals).
4. Populates `provenance.source_run_id` and leaves `provenance.reviewed_by` blank until a human fills it in and commits the assertions.

## 17.5 Verification

- Statistical correctness tests: feed the reliability engine synthetic pass/fail sequences with known true rates (e.g., a simulated 0.98 true-pass-rate generator) and confirm the Wilson interval and verdict logic classify `PASS`/`FAIL`/`FLAKY` correctly across many trials.
- An extraction test: confirm `gust scenario from-run` never populates `assertions` from the source run's actual behavior.
- A cost/latency test: N samples run in parallel where the `TestRunner` supports it, so `gust test` on a 20-sample scenario does not take 20x a single run's wall-clock time serially.

## 17.6 Cost and CI-strategy guidance (non-normative, but must be documented)

Live Test-mode sampling is the only part of the MVP that costs money or is slow (it calls a real LLM). Recommended default CI strategy, to be documented in `docs/`:

- Deterministic evaluators, Replay-mode mutation testing, and Analyze-mode checks run on **every commit** — fast, free, deterministic.
- Test-mode scenarios run with a **reduced sample count** (e.g., 5) on every PR for fast feedback, and a **full sample count** (e.g., 20+) on a merge-to-main or nightly schedule, where `FLAKY` verdicts from the reduced run get a chance to resolve to `PASS`/`FAIL` with more evidence.
- A local model (Ollama) is the recommended default for Test-mode sampling in open-source CI, to keep the full MVP demonstrable at $0 (§23).

## 17.7 Exit criteria

A `TestScenario` run through `gust test --samples 20` against a deliberately-flaky synthetic agent (passes ~85% of the time) is correctly classified `FLAKY` against a 95% `minimum_pass_rate`, and correctly classified `PASS` when re-run against a reliable synthetic agent (passes ~99% of the time) — proving the statistical machinery, not just the plumbing, works.

---

# 18. Policy Engine (extended for probabilistic verdicts)

```yaml
policy:
  require:
    task_success: true

  constraints:
    forbidden_tools: 0
    max_tool_calls: 8
    max_latency_ms: 5000

  reliability:                    # new — governs Test-mode scenarios
    minimum_pass_rate: 0.95
    on_flaky: warn                # warn | fail | ignore — CI behavior when a scenario is FLAKY

  regression:
    task_success_drop: <= 2%
    recovery_drop: <= 5%
```

```go
type PolicyEngine interface {
    Evaluate(results []EvaluationResult, reliability []ReliabilityResult, baseline *ExperimentResult) (Verdict, error)
}
```

`Verdict` now includes `PASS | FAIL | FLAKY`, plus which specific clauses (deterministic constraint, reliability threshold, or regression rule) produced that outcome, plus the underlying `EvaluationResult`/`ReliabilityResult` evidence. `on_flaky` lets a team decide whether a `FLAKY` scenario blocks CI (`fail`), merely warns (`warn`, the sensible default for most teams), or is ignored (`ignore`, for scenarios still being tuned).

Verification and exit criteria are unchanged in substance from v0.3 §16.4/§16.5, extended to include a golden test where the same evaluator + reliability result set, run through two different `on_flaky` settings, produces two different correct verdicts.

---

# 19. Structured Evidence & Failure Explanation

Unchanged from v0.3 §17, with `ReliabilityResult.per_run_evidence` (§17.3) as the Test-mode extension: a `FAIL` or `FLAKY` verdict must let a developer inspect *which* of the N sampled runs failed and why, not just the aggregate rate.

---

# 20. Dataset, Experiment & Regression Model

## 20.1 Dataset

A `DatasetCase` is now explicitly a `TestScenario` (§9) when it targets Test mode, or a plain `AgentRun` reference when it targets Analyze/Replay mode:

```json
{
  "name": "customer-support",
  "version": "1",
  "content_hash": "sha256:...",
  "cases": [
    { "kind": "test_scenario", "ref": "scenarios/cancel_latest_order.yaml" },
    { "kind": "recorded_run", "ref": "runs/run_18291.json" }
  ]
}
```

Immutability enforced via content hashing, unchanged from v0.3 §18.

## 20.2 Experiment

```text
Agent version + Dataset version (content hash) + Evaluator versions + Policy version + Fixture set version + reliability sampling config
```

## 20.3 Two distinct statistical questions — both required in the MVP

1. **Single-scenario reliability (§17):** "is this one `TestScenario`'s pass rate above threshold, given N samples of the *current* agent?" — a within-experiment question, always probabilistic for Test-mode scenarios.
2. **Baseline-vs-candidate regression (this section):** "did the aggregate pass rate across a whole dataset regress between two experiments?" — a between-experiment question, using a two-proportion or bootstrap comparison across the aggregated results of (1).

```text
                    baseline    candidate

task success         94.2%       95.1%   +0.9%
tool correctness     97.1%       96.8%   -0.3%
recovery             91.4%       74.2%  -17.2%  <- REGRESSION
latency               810ms       790ms   -2.5%

RESULT: FAIL
```

A minimal statistical gate (Wilson interval for pass-rate metrics, bootstrap CI for continuous metrics) ships in the MVP CLI/CI phase for **both** questions — do not fail CI on tiny statistically insignificant differences in either the single-scenario or the baseline-vs-candidate sense, from the first shipped version. A fuller engine (bootstrap comparisons, effect size, minimum sample size guidance) is a P1 upgrade of both gates, not a separate first appearance of statistics.

---

# 21. CLI & CI Requirements

Single static binary, Cobra-based:

```bash
gust analyze <run.json>                       # Mode 1 — no execution
gust replay <run.json>                        # Mode 2 — deterministic re-execution, no live LLM
gust test <scenario.yaml> [--samples N]       # Mode 3 — REAL agent + REAL LLM, probabilistic
gust mutate <agent.json> --classes all --n 100
gust compare <baseline> <candidate>
gust scenario from-run <run.json> --output <scenario.yaml>   # §17.4
```

CI support: policy-based gating including `on_flaky` behavior (§18), statistical significance gates for both single-scenario reliability and baseline/candidate regression (§20.3), machine-readable JSON output, exit codes, human-readable summaries with `FLAKY` rendered distinctly from `FAIL`.

```yaml
policy: default.yaml

reliability:
  default_samples: 10          # overridable per-scenario, §17.2
  min_samples_for_verdict: 5

significance:
  method: wilson
  confidence: 0.95
```

Exit criteria: a developer can add gust to a repository and gate an agent change in CI using the prebuilt binary; CI does not fail on statistically insignificant noise in either statistical sense (§20.3) from the first shipped version; a `FLAKY` result is visibly distinguished from `FAIL` in every output format.

---

# 22. Offline-Only Operation

Analyze, Replay, and Mutate work with zero network access, zero API key, zero cloud account — Go single static binary + synthetic/recorded fixtures + deterministic evaluators. **Test mode requires invoking some LLM**, but that LLM can be a $0 local model (Ollama) — see §23 — so the entire MVP, including the probabilistic Test-mode machinery, is demonstrable without a paid subscription.

---

# 23. Non-Functional Requirements

| Requirement | Target |
|---|---|
| CLI cold start (single Analyze/Replay run) | ≤ 200ms |
| Deterministic evaluator throughput (single core) | ≥ 1,000 cases/sec |
| Median mutation-suite evaluation time per case | ≤ 5ms (stretch: ~1.4ms) |
| Dataset of 10,000 Analyze/Replay cases, end-to-end | < 2 minutes on commodity CI hardware |
| Test-mode N-sample execution | samples run in parallel where the `TestRunner`/LLM provider allows it; wall-clock for N samples should not scale linearly with N on a rate-limit-permitting provider |
| Trace processing memory model | streaming/iterator-based |

Test-mode throughput is inherently bounded by LLM provider latency/rate limits, not by gust's own engine — this must be documented so users don't mistake provider latency for an gust performance problem.

---

# 24. Agent Evaluation Effectiveness (AEE) — north-star metric

Unchanged in shape from v0.3 §24 — a composite score **for evaluation suites**, not agents:

```text
AEE = f(mutation_detection_rate, false_positive_rate, reproducibility, evaluation_latency)
```

All four inputs are measured in Replay mode (deterministic, so the metric itself is reproducible) — Test-mode reliability (§17) is a property of a given *agent*, not of the *evaluation suite*, and is intentionally excluded from AEE to keep the two concepts (suite trustworthiness vs. agent reliability) from being conflated.

## 24.1 Aspirational public benchmark (P2, unchanged from v0.3 §24.1)

Numbers-only-when-measured; no invented competitor figures, ever.

---

# 25. OpenTelemetry / OpenInference as an Ingestion Path

Unchanged from v0.3 §25 — feeds Analyze mode (and, via the `TestScenario` extraction workflow, §17.4, can seed new Test-mode scenarios).

---

# 26. Security & Trust Model

Unchanged in substance from v0.3 §26, with one addition specific to v0.4:

- **`TestRunner` plugins are a stronger trust boundary than evaluators/mutators/fixture providers**, because they are the one plugin kind explicitly permitted to make live network calls (to the agent process and/or the LLM provider). A third-party `TestRunner` must be explicitly opted into per-project configuration (never auto-discovered/auto-run the way a sandboxed evaluator can be), and its network access is not restricted the way §7.12/§26's default sandboxing restricts other Tier 2 plugins.

---

# 27. Interface Versioning Policy

Unchanged from v0.3 §27, extended to cover `TestRunner` and `ReliabilityResult` under the same SemVer/`protocol_version` discipline.

---

# 28. Verification Strategy

Four levels from v0.3 §28, plus a fifth new to v0.4:

- **Unit / Contract / Integration / Benchmark tests:** unchanged in shape.
- **Reliability tests (new):** feed the §17.3 statistical engine synthetic pass/fail sequences with known true rates and confirm correct `PASS`/`FAIL`/`FLAKY`/`INSUFFICIENT_SAMPLES` classification across many trials (this is to the reliability engine what mutation testing is to the evaluator suite — validating the validator).

---

# 29. Phased Roadmap

| Phase | Objective | Exit criteria |
|---|---|---|
| **0 — Research & Spec** | Ecosystem comparison (§3); ADRs; license recorded. | Team can articulate all six differentiation points in §3, including the probabilistic-testing one, without hand-waving. |
| **1 — Core Schema** | `AgentRun`/`Span`/`Fixture`/`TestScenario`/`Assertion`/`Policy`/`EvaluationResult`/`ReliabilityResult` as JSON Schema. | Round-trip tests pass in Go and one other language, including `TestScenario` and `ReliabilityResult`. |
| **2 — Extensible SDK + Wire Protocol** | Evaluator/Mutator/FixtureProvider/**TestRunner** registry, both tiers; thin Python SDK. | Third-party evaluator/mutator/test-runner addable in Go and Python without core changes. |
| **3 — Deterministic Evaluator Suite** | The 10 MVP evaluators. | Synthetic-agent matrix passes; no paid AI service used. |
| **4 — Analyze & Replay Modes** | Fixture Store, failure-injection modes, deterministic re-execution, zero live calls. | 100 repeated Replay-mode runs of the same input produce identical results. |
| **5 — Mutation Engine (Replay mode)** | Mutators for the 9 MVP classes; Detection Rate + FPR. | ≥90% Detection Rate, ≤5% FPR, reproduced. |
| **6 — TestScenario & Probabilistic Test Mode** *(new flagship phase, pulled forward per this revision)* | `TestScenario`/`Assertion` schema; `TestRunner` interface (local-model backend first); §17's statistical reliability engine; `gust scenario from-run` extraction tooling. | The §17.7 exit criterion: correct `FLAKY` classification on a deliberately-flaky synthetic agent and correct `PASS` on a reliable one, via `gust test --samples 20`, against a $0 local model. |
| **7 — Policy Engine** | Policy schema incl. `reliability`/`on_flaky`; aggregation; verdict + evidence. | Same evaluator+reliability output through two different `on_flaky` settings yields two different correct verdicts. |
| **8 — CLI & CI** | `analyze`/`replay`/`test`/`mutate`/`compare`/`scenario` as a single static binary; both statistical gates (§20.3). | Repo can gate a PR in CI using the prebuilt binary; neither gate fails on insignificant noise; `FLAKY` renders distinctly. |
| **8.5 — Trust Boundary** | Sandboxing for Tier 2 evaluator/mutator/fixture-provider plugins; explicit opt-in model for `TestRunner` plugins (§26). | A misbehaving sandboxed plugin is contained; a `TestRunner` plugin's network access is documented and requires explicit config, never silent auto-run. |
| **9 — OTel/OpenInference Ingestion** | `ingest/otel/` adapter feeding Analyze mode and scenario extraction. | Externally-instrumented trace ingested, analyzed, and observable through standard OTel infra. |
| **10 — Framework Adapters** | Generic; one Python framework; one different-language/execution-model adapter. | Same evaluator suite comparable across multiple frameworks/languages. |
| **11 — Optional LLM-Based Evaluation** | `JudgeProvider`, local/mock backends first. | Stable only after clearing Spearman ρ ≥ 0.7 against a ≥50-case calibration set; otherwise "experimental." |
| **12 — Statistical Regression Engine v2** | Bootstrap, effect size, minimum sample size — upgrades both §17 and §20 gates. | Correctly distinguishes controlled `A==B`/`A slightly>B`/`A significantly>B` samples for both statistical questions. |
| **13 — Production Trace Evaluation & Automated Extraction** | Sampling/filtering feeding Analyze + automated `TestScenario` candidate proposals; PII redaction by default. | Production evaluation runs without evaluating every request or leaking unredacted PII; proposed scenarios still require human review before assertions are added (§17.4's safeguard holds at scale). |
| **14 — Failure Mining / Clustering** | Cluster similar production failures to reduce duplicate proposed scenarios. | ≥80% of repeated synthetic failure patterns grouped correctly. |
| **15 — Multi-Agent Evaluation** | Handoff/delegation/coordination evaluators, both Replay-mode mutation and Test-mode reliability variants. | Known multi-agent mutations detected; multi-agent scenarios get correct `FLAKY`/`PASS`/`FAIL` treatment. |
| **16 — AEE Public Benchmark** | Empirically measured comparison against DeepEval/Phoenix/etc. | Numbers published only once measured; methodology reproducible by a third party. |

---

# 30. Gold-Standard Criteria

Unchanged in substance from v0.3 §30, with one addition: **scientific criteria now explicitly include reliability-engine correctness** (§17.5/§28) alongside the mutation benchmark and any future judge correlation — a project claiming to test stochastic systems must be able to demonstrate its own statistical machinery classifies known-flaky and known-reliable synthetic agents correctly, not just that its deterministic evaluators catch synthetic mutations.

---

# 31. What We Need to Prove

## H1–H5
Unchanged from v0.3 §31 (trace normalization across languages; failure detection independent of final output; deterministic evaluation sufficiency; reproducibility; cross-language extensibility).

## H6 — Policy decoupling
Unchanged from v0.3.

## H7 — Probabilistic assertions correctly distinguish reliable, flaky, and broken agents (new, primary for v0.4)
Verification: against synthetic agents engineered to have known true pass rates (e.g., 99%, 85%, 40%), `gust test` classifies them `PASS`, `FLAKY`, and `FAIL` respectively (relative to a 95% threshold) at the configured sample size, across repeated trials of the whole experiment — i.e., the classifier itself is validated the same way the evaluator suite is validated in H3, not merely plumbed through.

## H8 — Extracted scenarios do not silently encode past mistakes (new)
Verification: `gust scenario from-run` never produces a `TestScenario` whose `assertions` match the source run's actual (possibly incorrect) behavior without human intervention — checked structurally (assertions field is empty/templated on extraction) rather than by trusting the tool's intent.

---

# 32. Recommended MVP Technology Strategy

Unchanged from v0.3 §32: **Go from Phase 0, no planned migration**, for the same reasons (CI-native single-binary distribution, determinism-first fits a compiled/typed language, concurrency for both Test-mode parallel sampling and future production-scale workloads, direct precedent in Open Policy Agent, no migration risk given a small domain model). The Tier 2 wire protocol (§13) remains the mechanism that keeps Python-only contributors fully able to add evaluators, mutators, fixture providers, and now `TestRunner`s without touching Go.

## Stack (unchanged from v0.3, `gonum` now also backing §17's Wilson interval computation)

```text
Go 1.23+                          core engine, CLI, all five execution-mode/engine components, evaluator suite
JSON Schema (Draft 2020-12)       spec/ — source of truth
santhosh-tekuri/jsonschema        Go-side schema validation
Cobra                             CLI framework
gonum                             Wilson intervals (§17), bootstrap/statistics (§20, Phase 12)
modernc.org/sqlite                pure-Go SQLite driver — only if persistence is needed
Go testing package + testify      unit/contract/integration/reliability tests
OpenTelemetry Go SDK              §25, Phase 9
Python 3.12+ (sdk/python/)        thin SDK: AgentRun/TestScenario construction + Tier 2 wrapper
```

Do not introduce Kubernetes, Kafka, Postgres, Redis, cloud infrastructure, or microservices during the MVP.

---

# 33. Cost-Constrained Development Plan

## Stage 1 — $0 (Analyze, Replay, Mutate)
Synthetic agents (Go) + recorded/synthetic fixtures + deterministic evaluators + mutation engine. No live services, no LLM, no cloud.

## Stage 2 — $0 (Test mode, MVP-required, not deferred)
A local model (Ollama/llama.cpp) reached over HTTP from a Go `TestRunner`, used for §17's probabilistic sampling. **This is part of the MVP, not a later addition** — the whole point of v0.4 is that probabilistic testing must be demonstrable from day one, and it must be demonstrable for $0.

## Stage 3 — optional, later
A paid OpenAI/Anthropic/Gemini-backed `TestRunner`, as a plugin, only when useful, never required to validate the architecture or the reliability-classification hypothesis (H7).

---

# 34. The MVP Killer Demo

Extended with a probabilistic Test-mode example — this is the artifact the README leads with.

```text
$ gust mutate agent.json --classes all --n 100

Generated 100 agent mutations.

Mutation results:

Tool selection          10/10 detected
Bad arguments             9/10 detected
Missing recovery          8/10 detected
Infinite loops           10/10 detected
Forbidden actions        10/10 detected
Excessive tool usage      9/10 detected

--------------------------------
Detection rate:          92%
False positive rate:      1.8%
Median evaluation:       1.4ms
--------------------------------
```

```text
$ gust test cancel_latest_order.yaml --samples 20

Running cancel_latest_order against support-agent:1.4 (local model: llama3.1:8b)

  17/20 passed  (observed pass rate: 85.0%)
  95% confidence interval: [64.0%, 95.0%]
  required minimum pass rate: 95.0%

VERDICT: FLAKY
  The agent is not reliably calling cancel_order(123) instead of cancel_order(122).
  3 of 20 runs called the wrong order. See --verbose for per-run evidence.
```

```text
$ gust compare baseline.json candidate.json

                 baseline    candidate

Task success       94.2%       95.1%   +0.9%
Tool correctness   97.1%       96.8%   -0.3%
Recovery           91.4%       74.2%  -17.2%  <- REGRESSION
Latency            810ms       790ms   -2.5%

RESULT: FAIL (policy: default.yaml, clause: regression.recovery_drop)
```

All three commands must work **offline except for the local-model call in `test`**, with zero configuration beyond the local files, on a fresh clone of the repository, before Phase 7 (§29) is considered done.

---

# 35. Final Project Definition

### One-line description

> **gust is test infrastructure for autonomous software: a language-neutral engine that turns production traces into reproducible test scenarios, runs real agents against controlled environments with honest probabilistic reporting, validates its own evaluators by mutation testing, and gates regressions with a policy engine and structured evidence.**

### Differentiation

```text
TEST-INFRASTRUCTURE-FIRST — not a metrics library; the mental model is JUnit + mocking + CI for agents
TRACE-IS-NOT-THE-TEST     — extraction from AgentRun to TestScenario is deliberate and human-reviewed
PROBABILISTIC-HONEST      — Test mode reports a sampled pass rate and confidence interval, never a bare n=1 boolean
MODE-SEPARATED            — Analyze / Replay / Test are architecturally distinct, never blurred
MUTATION-VALIDATED        — the evaluator suite's own reliability is measured (Replay mode) and published
POLICY-DECIDES            — evaluators measure; a separate policy engine issues PASS/FAIL/FLAKY
LANGUAGE-NEUTRAL          — enforced by a wire protocol, not asserted
CI-NATIVE                 — single static binary, statistically rigorous (in two distinct senses) from day one
OTEL-COMPATIBLE           — an ingestion path, not a competing telemetry model
```

### Long-term aspiration

> When someone builds an AI agent, gust is the layer they use to turn a production incident into a permanent, honestly-measured regression test — one that tells them not just "did it pass" but "how reliably does it pass, and would we have caught it if it had failed differently" — regardless of what language their agent or their evaluator is written in.

---

# 36. Definition of Done — MVP (v0.4)

- [ ] `AgentRun`/`Span`/`Fixture`/`TestScenario`/`Assertion`/`Policy`/`EvaluationResult`/`ReliabilityResult` exist as JSON Schema.
- [ ] Tier 1 and Tier 2 interfaces exist for evaluators, mutators, fixture providers, **and TestRunners**.
- [ ] Tier 2 reference implementation exists in two languages (Go, Python).
- [ ] The 10 MVP deterministic evaluators are implemented; no LLM-based evaluator exists in the MVP.
- [ ] **Mode 1 (Analyze) and Mode 2 (Replay) are implemented and architecturally distinct from Mode 3 (Test) in code, not just in docs.**
- [ ] Replay Engine: zero live network calls; all 5 failure-injection modes implemented; determinism proven (100 repeated runs -> identical results).
- [ ] Mutation Engine: 9 MVP classes; ≥90% Detection Rate and ≤5% False Positive Rate, reproduced.
- [ ] **`TestScenario`/`Assertion` schema implemented; `gust test` runs a real agent against a real (at minimum, local) LLM.**
- [ ] **Probabilistic reliability engine implemented (Wilson interval, `PASS`/`FAIL`/`FLAKY`/`INSUFFICIENT_SAMPLES`), validated against synthetic agents with known true pass rates (H7), not just plumbed through.**
- [ ] **`gust scenario from-run` extraction tooling exists and structurally never auto-populates assertions from source-run behavior (H8).**
- [ ] Policy Engine issues `PASS`/`FAIL`/`FLAKY` with `on_flaky` configurability; two different policies on the same inputs yield two different correct verdicts.
- [ ] Every failing built-in evaluator produces structured evidence; `FAIL`/`FLAKY` `ReliabilityResult`s expose per-run evidence.
- [ ] Datasets, fixture sets, and test scenarios are content-addressed.
- [ ] `gust analyze / replay / test / mutate / compare / scenario from-run` all work as a single static binary.
- [ ] CI integration includes statistical gates for both single-scenario reliability and baseline-vs-candidate regression, from the first release.
- [ ] The §34 killer demo (including the probabilistic `test` example) runs end-to-end against a $0 local model, on a fresh clone.
- [ ] Untrusted Tier 2 evaluator/mutator/fixture-provider plugins run sandboxed; `TestRunner` plugins require explicit opt-in, never silent auto-run.
- [ ] License (Apache-2.0) and `adr/` governance process exist from Phase 0.
- [ ] Documentation explains the three execution modes, the trace-is-not-the-test principle, and both extension tiers.
- [ ] The entire MVP — including Test mode — runs without a paid AI API.

---

# 37. References

[1] OpenTelemetry — GenAI agent and framework semantic conventions. https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/gen-ai-agent-spans.md
[2] OpenTelemetry — GenAI semantic conventions. https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/gen-ai-spans.md
[3] DeepEval — agent evaluation quickstart and metrics. https://deepeval.com/docs/getting-started-agents , https://deepeval.com/docs/metrics-introduction
[4] Braintrust — systematic evaluation, datasets, experiments, CI regression. https://www.braintrust.dev/docs/evaluate
[5] Braintrust — best practices for evaluating agents (stubbing external dependencies). https://www.braintrust.dev/docs/best-practices/agents
[6] Arize Phoenix — tool-selection/tool-invocation evaluators, release notes; documentation home. https://arize.com/docs/phoenix/release-notes/02-2026/02-01-2026-tool-selection-and-tool-invocation-evaluators , https://arize.com/docs/phoenix/
[7] OpenTelemetry semantic conventions overview. https://opentelemetry.io/docs/specs/semconv/

---

# 38. Changelog: v0.3 → v0.4

| # | Area | Change |
|---|---|---|
| 1 | Core principle (new) | "A trace is not the test. A trace is evidence from which a reproducible test scenario can be created." (§4.1) |
| 2 | New central section | Three Execution Modes — Analyze, Replay, Test — made architecturally explicit and never to be blurred (§5). |
| 3 | Thesis reframe | "Test infrastructure for autonomous software" (JUnit + mocking + CI, adapted for a stochastic component) replaces "execution/replay/mutation engine" as the primary framing (§1, §35). |
| 4 | MVP scope (major) | **Probabilistic, repeated-sample Test-mode assertions moved into the MVP** (was implicitly deferred/unaddressed in v0.3's single-shot Replay Engine). `assert x == true` is explicitly rejected as insufficient given LLM nondeterminism (§6, §17). |
| 5 | New domain object | `TestScenario` — input + environment + independently-authored assertions + reliability config + provenance (§9). Explicitly distinct from `AgentRun`. |
| 6 | New domain object | `ReliabilityResult` — pass rate, confidence interval, `PASS`/`FAIL`/`FLAKY`/`INSUFFICIENT_SAMPLES` verdict (§17.3). |
| 7 | New verdict state | `FLAKY`, alongside `PASS`/`FAIL`, as a first-class Policy Engine outcome (§18). |
| 8 | New interface | `TestRunner` (Tier 1 + Tier 2) — the only plugin kind permitted live network calls to agent/LLM, with a correspondingly stronger trust boundary (§13, §26). |
| 9 | New workflow | `gust scenario from-run` — assisted, human-reviewed extraction from a captured `AgentRun` into a `TestScenario`, structurally prevented from auto-encoding past mistakes as passing assertions (§17.4, H8). |
| 10 | Reclassified | v0.3's "Replay Engine" split into Mode 2 (Replay — deterministic, no live LLM, used by Mutation testing) and Mode 3 (Test — live agent + live LLM, probabilistic) — previously conflated under one "Replay Engine" (§5.4, §15). |
| 11 | Statistics (major) | Two distinct statistical questions now both required in MVP: single-scenario reliability (§17) and baseline-vs-candidate regression (§20.3) — previously only the latter was specified. |
| 12 | New hypotheses | H7 (probabilistic classifier correctness) and H8 (extraction doesn't encode past mistakes), added to §31. |
| 13 | CLI | Added `gust analyze`, `gust test`, `gust scenario from-run`; `gust run` retired in favor of mode-specific commands (§21). |
| 14 | Cost plan | Local-model-backed Test mode moved from "P1/later" to Stage 2 of the $0 MVP cost plan — explicitly required for the MVP demo, not optional (§33). |
| 15 | Everything else (Mutation Engine mechanics, schema-as-source-of-truth, Tier 1/2 wire protocol, Go-from-day-0, security/trust model baseline, interface versioning, content-addressed datasets, AEE metric, OTel ingestion) | Carried forward from v0.3 largely unchanged in substance. |
