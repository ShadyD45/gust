# gust: A Language-Neutral Replay, Mutation & Evaluation Engine for AI Agents
## Design, Requirements, Phased Roadmap, and Verification Plan

**Status:** v0.3 (supersedes v0.2)
**Project type:** Open-source infrastructure/library
**Reference implementation language:** Go, from Phase 0 (unchanged from v0.2 — see §32)
**License:** Apache-2.0
**Audience for this document:** intended to be handed directly to an implementation team/coding agent for planning and build-out. Where a section says "MVP," treat it as literally in scope for the first buildable milestone; everything else is sequenced but deferred.

> **This revision is a thesis pivot, not a patch.** v0.2 fixed a structural contradiction (Python ABCs claiming language neutrality) but was still, at its core, "a generic agent evaluation framework" — schema + evaluators + plugins + CLI + OTel + judges. Competitive research (§3) shows that surface is already well covered by DeepEval, Braintrust, and Arize Phoenix. **v0.3 narrows the thesis to what those systems do not make first-class: deterministic replay against a recorded world, mutation testing as the primary trust metric, a policy engine that separates "what evaluators measured" from "was this acceptable," and structured evidence instead of scores.** LLM-as-judge is removed from the MVP entirely — not deprioritized, removed — because it cannot be the thing that proves the architecture works. Full diff in §38.

---

# 1. Executive Summary

Agent systems are software systems with planning, tool use, retrieval, memory, retries, delegation, and multi-step execution. Evaluating only the final response is insufficient, and the industry mostly already agrees with that — DeepEval, Braintrust, and Phoenix all now do trajectory-level evaluation.

**What none of them treat as first-class is the following three-part discipline, taken from ordinary software testing:**

1. **Replay** — run the agent against a *recorded, deterministic version of the world* (recorded tool/API/DB responses), not live external services, so a test is repeatable.
2. **Mutation testing** — don't just measure "does the evaluator produce a score," measure "**can this evaluation suite actually detect a broken agent**," by deliberately injecting known defects and checking detection rate and false-positive rate.
3. **Policy, not score** — a single 0–1 number is not a verdict. A policy engine consumes multiple evaluators' structured output and produces a pass/fail decision with **evidence**, the same way a CI pipeline aggregates test results rather than reporting "78% of assertions were true."

> **gust is the execution engine and open protocol for replaying, mutating, and evaluating agent executions — built to prove, quantitatively, that its own evaluation suite is trustworthy, before it asks anyone to trust its scores.**

OpenTelemetry/OpenInference remain the ingestion path for traces already captured by other tooling (§25) — gust does not compete with or replace them.

---

# 2. Why This Project Exists

## 2.1 The current evaluation model, even in modern tools, is often still a black box

For ordinary software we have:

```text
source code -> unit tests -> integration tests -> deterministic execution -> assertions -> CI -> regression detection
```

For agents, even with modern trajectory-aware tools, the practical experience is often still:

```text
prompt -> run agent -> LLM judge -> 0.82 -> ???
```

The number is not self-evidently trustworthy, and it gets worse as the agent gets more complex:

```text
LLM -> planner -> tool -> retriever -> sub-agent -> tool -> retry -> LLM -> answer
```

The right question is not "was the answer good?" It is:

> **Did the system behave correctly across the entire execution, under conditions we can reproduce exactly, and can we prove our evaluators would have caught it if it hadn't?**

## 2.2 An agent may fail in ways the final answer doesn't reveal

An agent may:

- call the wrong tool;
- use invalid arguments;
- repeat a tool unnecessarily;
- enter a loop;
- fail to recover from an error;
- violate a plan;
- incur excessive latency or cost;
- access an inappropriate tool;
- succeed only accidentally, or only because a live external service happened to be up and returned what the test author expected that day.

That last item is why replay determinism (§14) is treated as a P0 requirement, not a nice-to-have: a test suite that calls live services is not reproducible, no matter how good its evaluators are.

---

# 3. Competitive Landscape & Real Differentiation

The space is materially more mature than "agent evaluation is an empty space." This section names names, because vague positioning against unnamed "existing tools" is not a defensible strategy.

| Project | What it already does well | Why it is not what we're building |
|---|---|---|
| **DeepEval** | End-to-end, trajectory, and component-level agent evaluation; tracing; CI/CD integration; tool correctness, argument correctness, task completion, step efficiency metrics. | A Python developer framework and metric library. It does not treat mutation-testing-of-the-evaluator-suite or deterministic replay-against-recorded-fixtures as a first-class, load-bearing feature of the product. |
| **Braintrust** | Datasets, immutable experiments, CI regression gating, production scoring, feedback-loop-to-dataset workflows; explicitly recommends stubbing external dependencies for deterministic agent testing. | Validates the *idea* of stubbing dependencies, but as a recommendation/pattern within a hosted platform, not as a formal recorded-fixture protocol with its own schema and mutation-based validation. |
| **Arize Phoenix** | Deterministic and LLM evaluators, agent tool-selection/invocation evaluators, native OpenTelemetry integration, reports large speedups via concurrency/batching. | Observability-first; OTel-integrated (which we adopt as an ingestion path, not a competitor); does not center mutation testing as the trust metric for its own evaluators. |

**Conclusion: do not build "another DeepEval, but in Go," and do not compete on evaluator count or metric breadth.** All three of the above already have more metrics than a new project can or should try to match on day one.

### What is actually undefended territory

1. **A public, falsifiable claim about the evaluation suite's own reliability** (mutation detection rate + false positive rate), reported as a first-class product metric, not a footnote in a validation blog post.
2. **A formal, versioned recorded-fixture/replay protocol** — not a "best practice," a schema and engine.
3. **A policy engine that is the actual pass/fail authority**, decoupled from any single evaluator's score.
4. **A cross-language wire protocol** (§12) so the engine is not tied to being "a Python framework," which is what all three competitors above are, structurally.
5. **Raw deterministic-evaluation throughput** as a stated, benchmarked number — not LLM-judge throughput, which is bottlenecked by provider latency and not a fair fight, but pure Go, no-network, no-database evaluation of already-captured traces.

None of these five are "more metrics." All five are structural/methodological, which is why they're defensible even against systems with more built-in evaluators.

---

# 4. Product Vision & Core Principle

## Core principle (unchanged from v0.2)

**Any agent should be able to produce an gust-compatible execution, regardless of model provider, agent framework, programming language, deployment environment, tool provider, or tracing backend.**

This is enforced by the two-tier extensibility model (§12), not asserted.

## Long-term architecture (revised — Replay/Evaluate/Mutate/Policy as the four pillars)

```text
                       gust Standard
                              |
                +-------------+-------------+
                |                           |
            AgentRun                   Evaluator
             Schema                     Protocol
                |                           |
                +-------------+-------------+
                              |
                       gust Core (Go)
                              |
          +-------------------+-------------------+
          |                   |                   |
        Replay             Evaluate             Mutate
   (recorded fixtures,   (deterministic +    (inject known defects,
    deterministic world)  optional judge,      measure detection
                           later phase)         rate + FPR)
          |                   |                   |
          +-------------------+-------------------+
                              |
                           Policy
                    (aggregates evidence,
                     issues the verdict)
                              |
                              v
                    Regression Result
                   (structured evidence,
                    not a bare score)
                              |
                +-------------+-------------+
                |                           |
               CI                      Production
```

Everything else — framework adapters, OTel/OpenInference ingestion, Python/TypeScript SDKs, judge providers, a UI — is an **extension** around this core, not part of it.

---

# 5. Goals

## P0 goals (MVP — must exist for the project to be considered real)

1. Define a stable core execution model (`AgentRun`, `Span`, `Fixture`).
2. Provide an extensible evaluator interface — both Tier 1 (native Go) and Tier 2 (language-agnostic wire protocol).
3. **Deterministic evaluation only — no LLM judge in the MVP** (see §6, §13).
4. Evaluate complete trajectories, not just final outputs.
5. **A working Replay Engine that runs an agent against recorded fixtures instead of live external services (§14).**
6. **A working Mutation Engine that injects known defects into a known-good agent and measures Detection Rate + False Positive Rate (§15) — this is the project's primary validation method and its first public demo.**
7. **A Policy Engine that aggregates evaluator output into a single pass/fail verdict with structured evidence (§16, §17).**
8. Reproducible, content-addressed datasets and experiments.
9. Regression comparison with statistical significance from the first release.
10. CLI and CI integration, distributed as a single static binary.
11. Fully offline operation — no API key, no cloud account, no paid inference required to prove the architecture works.
12. A minimal security/trust model for third-party evaluators before any plugin ecosystem opens.

## P1 goals

1. OpenTelemetry/OpenInference ingestion adapter (§25) — consume existing traces, do not require re-instrumentation.
2. Framework adapters, including at least one non-Python/non-Go-only language, to test the wire protocol.
3. **Optional LLM-based evaluation, added only after P0's deterministic/replay/mutation stack is proven** (§6, explicit reversal of ordering vs. v0.2, where it was already deferred but still present in the roadmap earlier).
4. Production trace ingestion, PII-safe by default, feeding the Replay Engine's fixture recorder.
5. Failure mining from production into regression fixtures.
6. Statistical Regression Engine v2 (bootstrap, effect size, minimum sample size).
7. Multi-agent evaluation (handoff, delegation, coordination).
8. Human annotation/calibration, gating any future judge provider's promotion to "stable."

## P2 goals

1. **Agent Evaluation Effectiveness (AEE) public leaderboard (§24)** — benchmark gust's own detection/FPR/speed against DeepEval, Phoenix, and other suites, empirically, with numbers only published once actually measured.
2. Community-contributed evaluators, sandboxed and checksummed.
3. Cross-framework benchmark results.
4. Standard semantic conventions contribution back to OpenTelemetry GenAI conventions.
5. Evaluation registry.
6. Production continuous evaluation.
7. Safety/security evaluator ecosystem.

---

# 6. Non-Goals

The MVP must not attempt to:

- build an agent runtime;
- build an LLM;
- compete with model providers;
- **replace or duplicate OpenTelemetry/OpenInference — consume their traces instead (§25);**
- build a large hosted dashboard;
- provide hundreds of metrics — depth over breadth, per §3;
- claim a single universal "intelligence score";
- require paid AI APIs;
- require a cloud account;
- **include an LLM-as-judge evaluator at all.** This is an explicit, temporary non-goal, not an oversight: adding a judge before the deterministic/replay/mutation stack is proven would make the system *look* more sophisticated without proving anything, and would reintroduce cost, latency, variance, and provider dependence into the one part of the system whose entire value proposition is determinism (§2, §13).
- become a multi-language monorepo before the schema is stable.

---

# 7. Design Principles

## 7.1 Offline-first
The core must work without OpenAI, Anthropic, Gemini, paid inference, or internet access.

## 7.2 Deterministic-first
If a property can be checked deterministically, do not use an LLM judge. Given `AgentRun` + `ExpectedBehavior`, attempt schema checks, tool checks, state checks, trajectory checks, invariants, resource limits, and policy checks *before* ever reaching for a judge — and in the MVP, there is no judge to reach for.

## 7.3 Replay-determinism (new, promoted from an implementation detail to a core principle)
An agent execution under evaluation must run against a **recorded, deterministic version of the world** — recorded tool/API/DB/search/payment responses — not live external services. Production:

```text
Agent -> DB, API, Search, Payment (live)
```

becomes, under replay:

```text
Agent -> gust Fixture Store -> recorded DB response, recorded API response, ...
```

A test that depends on a live external service being up and returning today what it returned when the test was written is not a regression test. This principle is what makes mutation testing (7.4) trustworthy: mutations must be attributable to the injected defect, not to external nondeterminism.

## 7.4 Mutation-validated evaluation (new, promoted to a core principle)
An evaluator suite's own reliability must be demonstrated, not asserted. The mechanism is: take a known-good agent, inject a known defect, and check whether the suite detects it — while also checking it does **not** flag the known-good agent. Report both numbers (§15) as a first-class, publicly stated product metric, refreshed on every CI run of the benchmark suite itself.

## 7.5 Policy decides, evaluators measure (new, promoted to a core principle)
An individual evaluator answers a narrow, well-defined question ("was the expected tool called?"). It does not decide whether the run is acceptable. A separate **Policy Engine** (§16) consumes the full set of evaluator results and applies declared constraints, thresholds, and requirements to produce the actual pass/fail verdict. This mirrors how a CI pipeline aggregates individual test results rather than asking one test to speak for the whole suite.

## 7.6 Evidence over scores (new, promoted to a core principle)
`EvaluationResult` must carry structured, human-readable evidence (expected vs. actual, offending spans, unnecessary calls) sufficient to explain *why* a run failed without re-running it (§17). A bare numeric score is not an acceptable terminal output for a failing evaluator.

## 7.7 Model-based evaluation is a plugin, deferred
LLM-as-a-judge, when it is eventually added (P1, §5), must be an optional evaluator backend reached through the same Tier 1/Tier 2 abstraction as any other evaluator. It must never be hard-coded into the core, and it must never be required for the MVP's mutation-detection claims to hold.

## 7.8 Trace-first architecture
Evaluation consumes a normalized AgentRun/Trace model, separating instrumentation from evaluation from framework/runtime specifics.

## 7.9 OpenTelemetry/OpenInference compatible, not competing
Reuse OpenTelemetry/OpenInference concepts and semantic conventions wherever applicable (§25). gust adds replay, mutation, and evaluation-specific semantics; it does not fork or duplicate telemetry semantics.

## 7.10 Extensible by design — two tiers (unchanged from v0.2, still load-bearing)

### Tier 1 — in-process, native Go

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
}
```

Registering a new evaluator, mutator, or fixture provider must not require changes to the core engine:

```go
registry.RegisterEvaluator(MyEvaluator{})
registry.RegisterMutator(MyMutator{})
```

### Tier 2 — cross-language, wire protocol

Same JSON-over-stdio (gRPC later) contract as v0.2 (§12), extended to cover mutators and fixture providers, not just evaluators — so a Python-authored mutation strategy or a TypeScript-authored fixture recorder can participate without touching Go.

## 7.11 Stable core, experimental extensions
`core/`, `contrib/`, `experimental/` — only stable interfaces enter `core/` (see §9 for the corrected tree).

## 7.12 Security & trust model
Untrusted evaluators/mutators loaded via Tier 2 run sandboxed (resource limits, no network by default). Production trace ingestion redacts PII by default. Nothing from failure mining enters a dataset without human approval, and nothing enters a public benchmark without also passing a redaction pass. (Full detail in §26, unchanged in substance from v0.2 §7.8.)

## 7.13 Interface versioning policy
Go interfaces and the wire protocol follow SemVer independently of `schema_version`. MAJOR bumps require a migration guide and a deprecation window. (Unchanged from v0.2 §7.9.)

---

# 8. High-Level Architecture

```text
+------------------------------------------------------------------+
|                       Agent Application                          |
|              (Go / Python / TypeScript / anything)               |
+-------------------------------+----------------------------------+
                                |  instrumentation, or OTel/OpenInference ingestion (§25)
                                v
+------------------------------------------------------------------+
|                     gust SDK (per language)                 |
|         Trace API | Context | Schema validation | Dataset API    |
+-------------------------------+----------------------------------+
                                |  canonical AgentRun (JSON)
                                v
+------------------------------------------------------------------+
|             gust Core Engine  (Go, single static binary)    |
|                                                                    |
|  +-----------+   +-----------+   +-----------+   +-------------+  |
|  |  Replay   |-->| Evaluate  |-->|  Mutate   |-->|   Policy    |  |
|  | (fixtures,|   | (Tier 1/2 |   | (inject   |   | (aggregate, |  |
|  |  determ.  |   | evaluators|   | defects,  |   |  verdict +  |  |
|  |  world)   |   |  §12)     |   | measure   |   |  evidence)  |  |
|  |           |   |           |   | detect/FPR|   |             |  |
|  +-----------+   +-----------+   +-----------+   +-------------+  |
+---------------+--------------------------------+------------------+
                |                                  |
                v                                  v
      Tier 1: native Go evaluators/       Tier 2: Wire Protocol
      mutators/fixture providers          JSON-over-stdio / gRPC
                                                    |
                                     evaluators, mutators, fixture
                                     providers, and (P1+) judge
                                     providers in any language
                           |
                           v
              Evaluation Result (structured evidence, §17)
                           |
             +-------------+-------------+-------------+
             |             |             |              |
            CLI           CI       OTel Exporter   Public benchmark
       (Cobra, Go)   (policy-gated,  (native OTel      / AEE leaderboard
                       significance   Go SDK)           (P2, §24)
                       tested, §20)
```

---

# 9. Repository Architecture

```text
gust/
|
+-- spec/                        # the standard itself — language-neutral
|   +-- schemas/                 #   JSON Schema (Draft 2020-12): AgentRun, Span, Fixture,
|   |                             #   Policy, EvaluationResult, MutationResult
|   +-- semantic-conventions/
|   +-- evaluator-protocol/      #   Tier 2 wire protocol (evaluators, mutators, fixtures)
|   +-- versioning.md
|   +-- compatibility.md
|
+-- core/                        # Go — stable
|   +-- run/
|   +-- trace/
|   +-- replay/                  #   Replay Engine: fixture store, deterministic world (§14)
|   +-- evaluation/
|   +-- mutate/                  #   Mutation Engine: mutators, detection/FPR scoring (§15)
|   +-- policy/                  #   Policy Engine: aggregation, verdict, evidence (§16, §17)
|   +-- dataset/
|   +-- experiment/
|   +-- result/
|   +-- errors/
|
+-- evaluators/                  # Go — stable, built-in (Tier 1), §13
+-- mutators/                    # Go — stable, built-in mutation strategies, §15
|
+-- providers/
|   +-- fixture/                 #   recording/replay backends (§14)
|   +-- judge/                   #   reserved interface; no built-in implementation until P1 (§7.7)
|   +-- storage/
|   +-- exporter/
|
+-- cli/                         # Go, Cobra — stable: run, replay, evaluate, compare, mutate
|
+-- sdk/
|   +-- python/                  # thin SDK: AgentRun construction + Tier 2 wrapper
|   +-- typescript/              # added once Python SDK proves the pattern (P1)
|
+-- ingest/
|   +-- otel/                    # OpenTelemetry/OpenInference ingestion adapter (§25)
|
+-- contrib/                     # explicitly unstable
|   +-- adapters/                #   langgraph, openai, crewai, ...
|   +-- evaluators/               #   community-contributed, sandboxed per §26
|
+-- experimental/                # multi-agent, production continuous eval, etc.
|
+-- benchmarks/
|   +-- synthetic/                # synthetic agent + fixture generator
|   +-- mutation/                 # mutation test suite (the killer demo, §15, §34)
|   +-- golden/
|   +-- calibration/              # reserved for future judge correlation work (P1)
|
+-- examples/
+-- tests/
|   +-- unit/
|   +-- integration/
|   +-- contract/
|   +-- benchmark/
|   +-- mutation/
|   +-- replay/                   # fixture round-trip / determinism tests
|
+-- docs/
+-- adr/
|
+-- CONTRIBUTING.md
+-- LICENSE
+-- README.md
```

---

# 10. Core Domain Model

Defined first as **JSON Schema** in `spec/schemas/` (source of truth); Go structs are generated from or validated against it.

```text
AgentRun
Trace
Span
ToolCall
Fixture / RecordedResponse
Task
Dataset
DatasetCase
Evaluator
EvaluationResult (with structured Evidence, §17)
Mutator
MutationResult
Policy
Experiment
Comparison
Regression
```

## AgentRun

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

## Span

```json
{
  "span_id": "span_1",
  "parent_id": null,
  "type": "tool",
  "name": "get_order",
  "start_time": "...",
  "end_time": "...",
  "status": "success",
  "input": {},
  "output": {},
  "attributes": {}
}
```

## Fixture (new)

A recorded external interaction, keyed so the Replay Engine (§14) can serve it back deterministically:

```json
{
  "fixture_id": "fx_001",
  "tool": "get_order",
  "input_hash": "sha256:...",
  "recorded_input": { "order_id": "123" },
  "recorded_response": { "status": "success", "body": { "order_id": "123", "amount": 42.50 } },
  "recorded_at": "2026-06-01T12:00:00Z",
  "mode": "success"
}
```

`mode` supports the failure-injection modes defined in §14.2: `success`, `timeout`, `malformed`, `slow`, `partial_failure`.

---

# 11. Span Types

MVP: `agent`, `llm`, `tool`, `retrieval`, `memory`, `plan`, `error`.
Future: `agent_handoff`, `workflow`, `human_intervention`, `guardrail`, `sandbox`, `browser`, `code_execution`, `evaluation`.

These map to OpenTelemetry GenAI semantic conventions where possible (§25).

---

# 12. Evaluator Architecture & Wire Protocol

Native Go interface (Tier 1):

```go
type Evaluator interface {
    Name() string
    Version() string
    Evaluate(run AgentRun, expected *ExpectedBehavior, ctx EvaluationContext) (EvaluationResult, error)
}
```

Cross-language contract (Tier 2) — JSON over stdio, versioned via `protocol_version`:

```text
Request:
{
  "protocol_version": "1.0",
  "kind": "evaluator",              // or "mutator", "fixture_provider"
  "name": "tool_correctness",
  "version": "1.0.0",
  "run": { ...canonical AgentRun JSON... },
  "expected": { ... },
  "context": { ... }
}

Response:
{
  "score": 1.0,
  "passed": true,
  "reason": "Expected tool was called.",
  "evidence": [ ... structured evidence, §17 ... ],
  "metadata": {}
}
```

The registry treats Tier 1 and Tier 2 identically:

```go
registry.RegisterEvaluator(MyEvaluator{})            // Tier 1
registry.RegisterExternalEvaluator("evaluator.yaml") // Tier 2
```

A reference Tier 2 implementation ships in at least two languages (Go, Python) before Phase 2 is complete, so a Python-only contributor can write `class MyEvaluator(Evaluator): ...` against the thin Python SDK and have it work without touching Go.

---

# 13. MVP Evaluators (deterministic only — no LLM judge, §6)

| Evaluator | Question it answers |
|---|---|
| `TaskSuccess` | Did the task succeed per the declared outcome contract? |
| `ToolSelection` | Was the expected tool (or sequence of tools) called? |
| `ToolArguments` | Were tool call arguments correct? |
| `ToolSequence` | Was the trajectory efficient — no unnecessary/repeated calls? |
| `ForbiddenTool` | Was a prohibited tool avoided? |
| `RequiredTool` | Was a required tool called? |
| `MaxSteps` | Was execution within an allowed step/call budget? |
| `MaxLatency` | Was execution within a latency budget? |
| `ErrorRecovery` | Did the agent recover correctly from an injected failure (via a `timeout`/`malformed`/`partial_failure` fixture, §14.2)? |
| `SchemaValidation` | Does the final output satisfy a deterministic output schema? |

All ten work without any paid AI service and without any network access, by construction (they consume an already-captured `AgentRun`; they do not call out).

`ResponseQuality` (a subjective, LLM-based evaluator) is explicitly **not** part of the MVP. It is reserved as a P1 addition (§5, §7.7) once the deterministic/replay/mutation stack has proven itself.

---

# 14. Replay & Simulation Engine

## 14.1 Why this is P0, not an implementation detail

An evaluation is only as trustworthy as the conditions it ran under. If `get_order` hits a live API, a test failure could mean "the agent regressed" or it could mean "the API was flaky today" — and without replay, there is no way to tell which. The Replay Engine makes the second cause structurally impossible during evaluation.

```text
production:
Agent -> DB
       -> API
       -> Search
       -> Payment

replay:
Agent -> gust Fixture Store -> recorded DB response
                                  -> recorded API response
                                  -> recorded Search response
                                  -> recorded Payment response
```

## 14.2 Failure-injection modes

The Fixture Store does not only replay clean recorded responses — it can serve controlled failure conditions, so evaluators like `ErrorRecovery` (§13) have something real to detect:

| Mode | Behavior |
|---|---|
| `success` | Replay the recorded response as-is. |
| `timeout` | Simulate the call hanging past a configured deadline. |
| `malformed` | Return a recorded-but-corrupted response body. |
| `slow` | Return the recorded response after an injected latency. |
| `partial_failure` | For a multi-call sequence, succeed on early calls and fail on a specified later one (e.g. `get_user` succeeds, `update_user` times out). |

## 14.3 Recording workflow

```text
production or staging run
       |
       v
   capture (via SDK instrumentation or OTel ingestion, §25)
       |
       v
   Fixture (recorded_input, recorded_response, keyed by input_hash)
       |
       v
   Fixture Store (content-addressed, versioned like datasets, §18)
```

## 14.4 Interface

```go
type FixtureProvider interface {
    Lookup(call ToolCall) (RecordedResponse, bool, error)
    Mode(call ToolCall) FailureMode
}
```

Fixture providers are Tier 1/Tier 2 like evaluators (§12) — a recorded-fixture set can be produced and served from any language.

## 14.5 Verification

- A fixture round-trip test: record a call, replay it, confirm byte-identical response.
- A determinism test: run the same `AgentRun` + fixture set 100 times, confirm identical evaluation results every time (modulo explicitly declared nondeterminism, e.g. wall-clock timestamps).
- A failure-injection test per mode in §14.2, confirming `ErrorRecovery` observes the intended condition.

## 14.6 Exit criteria (Phase — see §29)

An agent can be replayed against a recorded fixture set with zero live network calls, and failure-injection modes are demonstrated to trigger the intended agent behavior deterministically across repeated runs.

---

# 15. Mutation Testing Engine (the project's primary trust metric)

## 15.1 Why this is the center of the project, not a QA afterthought

The core question is not "does the evaluator give a score?" It is:

> **Can this evaluation suite actually detect a broken agent?**

```text
Known-good agent
       |
       v
gust Mutator
       |
       +-- remove required tool
       +-- wrong tool
       +-- corrupt arguments
       +-- duplicate call
       +-- infinite loop
       +-- skip recovery
       +-- wrong handoff
       +-- violate policy
       |
       v
Broken agents
       |
       v
Evaluation suite (Replay + Evaluate + Policy)
       |
       v
Did it catch them?
```

## 15.2 Metrics — always reported as a pair

```text
Detection Rate = detected injected failures / total injected failures
False Positive Rate = healthy (GOOD-class) executions incorrectly failed / total GOOD-class executions
```

A high Detection Rate alone is gameable by a suite that fails everything. Both numbers are reported together on every CI run of the benchmark suite itself, not measured once at MVP sign-off.

**Target for MVP sign-off:** Detection Rate ≥ 90%, False Positive Rate ≤ 5%, across the supported mutation classes (§15.3).

## 15.3 Mutation classes (MVP)

```text
remove_required_tool
wrong_tool
corrupt_argument
duplicate_call
infinite_loop
skip_recovery
excessive_tool_calls
introduce_forbidden_tool
change_final_output
```

## 15.4 Interface

```go
type Mutator interface {
    Name() string
    Mutate(run AgentRun) (MutatedRun, error)
}
```

Registered and run identically to evaluators (Tier 1/Tier 2, §12). Community-contributed mutation strategies are a P2 goal (§5), sandboxed per §26 like any other external plugin.

## 15.5 CLI surface

```bash
gust mutate agent.json --classes all --n 100
```

Output shape is defined in the MVP killer demo (§34) — this command *is* the flagship first-run experience of the project.

## 15.6 Verification

- Run the full mutation suite against the synthetic reference agent, living under `benchmarks/synthetic/`.
- Confirm each mutation class is independently attributable: a `wrong_tool` mutation should be caught primarily by `ToolSelection`, not accidentally by `MaxLatency`, so failures are diagnosable, not just detected.

## 15.7 Exit criteria

≥90% Detection Rate and ≤5% False Positive Rate, reproduced on repeated runs (tying back to §14.5's determinism requirement — a flaky mutation suite is not a validated one).

---

# 16. Policy Engine

## 16.1 Why evaluators don't get the final word

A single evaluator answers one narrow question. Whether a run is *acceptable* is a separate, declarative decision that should live in configuration, not in an evaluator's internals:

```yaml
policy:
  require:
    task_success: true

  constraints:
    forbidden_tools: 0
    max_tool_calls: 8
    max_latency_ms: 5000

  quality:
    final_answer_schema_valid: true

  regression:
    task_success_drop: <= 2%
    recovery_drop: <= 5%
```

## 16.2 Interface

```go
type Policy struct {
    Require     map[string]bool
    Constraints map[string]Constraint
    Regression  map[string]RegressionRule
}

type PolicyEngine interface {
    Evaluate(results []EvaluationResult, baseline *ExperimentResult) (Verdict, error)
}
```

`Verdict` includes the pass/fail decision, which specific policy clauses failed, and the `EvaluationResult`/`Evidence` (§17) that justifies each one.

## 16.3 Relationship to the CI significance gate (§20)

The Policy Engine is the *rule* layer (declarative, per-run). The statistical significance gate (§20) is the *noise* layer (is this difference real, across a sample). Both must pass for a regression check to fail CI: a policy violation on a single flaky sample without statistical backing should not gate a merge, and a statistically significant regression against a metric no policy declared as required should be reported but not block.

## 16.4 Verification

Golden policy files with known evaluator result sets, asserting the expected verdict and the expected set of failing clauses.

## 16.5 Exit criteria

The same evaluator output, run through two different policy files, produces two different (correct) verdicts — proving the policy is genuinely decoupled from evaluator internals.

---

# 17. Structured Evidence & Failure Explanation

## 17.1 Why a score is not an acceptable terminal output

`Agent score: 0.71` is not actionable. The required output shape is closer to:

```text
FAILED

TaskSuccess         PASS
ToolSelection       PASS
ArgumentCorrectness FAIL
  expected: user_id = "123"
  actual:   user_id = "132"
ToolSequence        FAIL
  unnecessary calls: search_user x3
ErrorRecovery       PASS
MaxLatency          PASS
```

## 17.2 Evidence schema (extends `EvaluationResult` from §12)

```json
{
  "evaluator": "argument_correctness",
  "evaluator_version": "1.0",
  "score": 0.0,
  "passed": false,
  "reason": "Argument mismatch on user_id.",
  "evidence": [
    {
      "span_id": "span_4",
      "field": "input.user_id",
      "expected": "123",
      "actual": "132",
      "kind": "value_mismatch"
    }
  ],
  "metadata": {}
}
```

`evidence` entries are structured (span reference + field + expected/actual + a `kind` enum), not free text, so downstream tooling (CLI report, CI annotation, IDE plugin) can render them without re-parsing prose.

## 17.3 Exit criteria

Every built-in MVP evaluator (§13) that can fail produces at least one structured evidence entry on failure — a passing evaluator with no evidence is fine; a *failing* evaluator with an empty evidence array is a bug.

---

# 18. Dataset Architecture

```json
{
  "name": "customer-support",
  "version": "1",
  "content_hash": "sha256:...",
  "cases": [
    {
      "id": "refund-001",
      "input": "Refund order 123",
      "fixtures": ["fx_001", "fx_002"],
      "expected": {
        "required_tools": ["get_order", "refund_order"],
        "forbidden_tools": ["delete_user"]
      }
    }
  ]
}
```

Requirements: immutability enforced via content hashing (a version+content mismatch is a hard error, not a silent overwrite), stable case IDs, metadata, tags, provenance, deterministic loading, reproducible execution. Each case may reference the Fixture set (§14) it should be replayed against.

---

# 19. Experiment Model

```text
Agent version + Dataset version (content hash) + Evaluator versions + Policy version + Fixture set version
```

```text
experiment:
    agent: support-agent:1.4
    dataset: support:v3 (sha256:9f2a...)
    policy: default:v1
    evaluators:
        - task_success:v1
        - tool_correctness:v1
        - tool_sequence:v1
```

Sufficient to reproduce an experiment exactly, including re-fetching the exact dataset and fixture content by hash.

---

# 20. Regression Model & Statistical Gate

```text
                    baseline    candidate

task success         94.2%       95.1%   +0.9%
tool correctness     97.1%       96.8%   -0.3%
recovery             91.4%       74.2%  -17.2%  <- REGRESSION
latency               810ms       790ms   -2.5%

RESULT: FAIL
```

The system must distinguish absolute threshold failure, statistically significant regression, noise/inconclusive, and improvement. **A minimal statistical gate (Wilson score interval for pass-rate metrics, bootstrap CI for continuous metrics) ships in the MVP CLI/CI phase (§21), not deferred to a later statistics-specific phase** — do not fail CI on tiny statistically insignificant differences, from the first shipped version. A fuller engine (bootstrap comparisons, effect size, minimum sample size guidance, `INCONCLUSIVE` state) is a P1 upgrade of this same gate, not a separate first appearance of statistics.

---

# 21. CLI & CI Requirements

Single static binary, Cobra-based:

```bash
gust run <task-or-dataset>       # execute an agent against a dataset (with replay fixtures)
gust replay <run.json>           # re-run a captured AgentRun deterministically against its fixtures
gust evaluate <run.json>         # run the evaluator suite + policy against a captured run
gust compare <baseline> <candidate>
gust mutate <agent.json>         # the flagship mutation-testing command, §15.5, §34
```

CI support: policy-based gating (§16), statistical significance gates (§20), machine-readable JSON output, exit codes, human-readable summaries.

```yaml
policy: default.yaml

significance:
  method: wilson       # or bootstrap for continuous metrics
  confidence: 0.95
  min_sample_size: 30
```

Exit criteria: a developer can add gust to a repository and gate an agent change in CI using the prebuilt binary (no interpreter/runtime install step), and CI does not fail on statistically insignificant noise from the first shipped version.

---

# 22. Offline-Only Operation

The entire MVP works with: Go (single static binary) + synthetic agents + recorded fixtures + JSON datasets + deterministic evaluators + the mutation engine. No API key, cloud, paid model, or database is required. This is easier to guarantee than in a Python virtualenv, since there is no dependency-resolution step to reach a package index.

---

# 23. Non-Functional Requirements

| Requirement | Target |
|---|---|
| CLI cold start (single-run evaluation) | ≤ 200ms |
| Deterministic evaluator throughput (single core) | ≥ 1,000 cases/sec |
| Median mutation-suite evaluation time per case | ≤ 5ms (stretch target from the killer demo, §34: ~1.4ms) |
| Dataset of 10,000 cases, deterministic evaluators only, end-to-end | < 2 minutes on commodity CI hardware |
| 100,000 `AgentRun`s, deterministic evaluation, no network/DB/LLM | single-digit seconds, not minutes |
| Trace processing memory model | streaming/iterator-based; no full-trace double-buffering |

These are placeholders for the team to tune against real measurements, but the benchmark suite must report against them from the mutation-engine phase onward (§15, §29).

---

# 24. Agent Evaluation Effectiveness (AEE) — the north-star metric

Rather than a single "intelligence score" for agents (explicitly a non-goal, §6), gust defines a composite score **for evaluation suites themselves**:

```text
AEE = f(mutation_detection_rate, false_positive_rate, reproducibility, evaluation_latency)
```

Exact weighting is an open research question and must not be finalized speculatively — but the four inputs are fixed from the MVP:

- **Detection Rate** (§15.2)
- **False Positive Rate** (§15.2)
- **Reproducibility** — variance of results across repeated runs of the same `AgentRun` + fixture set (§14.5); ideally zero for deterministic evaluators
- **Evaluation latency** — median time per case (§23)

## 24.1 Aspirational public benchmark (P2 — do not publish comparative numbers before they are actually measured)

```text
                    Detection    False+    Median time
gust             ?%           ?%         ?ms
DeepEval              ?%           ?%         ?ms
Phoenix               ?%           ?%         ?ms
```

This table is a target shape, not a claim. Populating it against real competitor installations, run fairly and reproducibly, is explicit P2 scope (§5) — publishing invented or estimated numbers for competitors is out of the question and must never happen, including informally in marketing copy.

---

# 25. OpenTelemetry / OpenInference as an Ingestion Path (not a competitor)

```text
existing traces (OpenTelemetry / OpenInference)
       |
       v
   ingestion adapter (ingest/otel/)
       |
       v
      AgentRun
       |
       v
   gust (Replay / Evaluate / Mutate / Policy)
```

If a team already instruments with OpenTelemetry (directly or via Phoenix/OpenInference), gust should be able to consume those traces without requiring re-instrumentation. gust-specific concepts (replay fixtures, mutation results, policy verdicts, experiment/regression records) are exported *back* alongside standard OTel spans (§28's Phase 8, unchanged from v0.2's OTel phase in substance), not invented as a competing telemetry model.

---

# 26. Security & Trust Model

(Unchanged in substance from v0.2 §7.8, extended to cover mutators and fixture providers.)

- Untrusted Tier 2 evaluators/mutators/fixture providers run in a resource-limited subprocess: CPU cap, memory cap, wall-clock timeout, no network access by default.
- Production trace ingestion redacts/hashes configurable PII fields by default; raw content passthrough is opt-in, never opt-out.
- Nothing from failure mining enters a dataset without human approval; nothing enters the public benchmark (§24.1) without also passing a redaction pass.
- Registry entries (P2) are namespaced and checksummed before gust will fetch and run them automatically.

---

# 27. Interface Versioning Policy

(Unchanged from v0.2 §7.9.) Go interfaces and the wire protocol (`protocol_version`) follow SemVer independently of `schema_version`. MAJOR bumps require a migration guide and a minimum one-MINOR-release deprecation window.

---

# 28. Verification Strategy

Four levels, extended for replay/mutation/policy:

- **Unit tests**: schemas, evaluators (both tiers), mutators, fixture providers, policy aggregation, statistical logic, serialization, versioning. Target ≥90% meaningful core coverage; coverage alone is not sufficient.
- **Contract tests**: every adapter emits a schema-valid `AgentRun`; every evaluator produces a schema-valid `EvaluationResult` with evidence on failure (§17.3), regardless of tier.
- **Integration tests**: agent -> SDK -> replay -> evaluator engine -> policy -> result -> CLI.
- **Benchmark tests**: full synthetic/mutation suite, reporting Detection Rate, False Positive Rate, reproducibility, and latency together (the four AEE inputs, §24) on every run.

---

# 29. Phased Roadmap

| Phase | Objective | Exit criteria |
|---|---|---|
| **0 — Research & Spec** | Ecosystem comparison (§3) named against DeepEval/Braintrust/Phoenix specifically; ADRs started in `adr/`; license (Apache-2.0) recorded. | Team can articulate the five-point differentiation in §3 without hand-waving; JSON Schema draft for `AgentRun`, `Span`, `Fixture` exists. |
| **1 — Core Schema** | `AgentRun`/`Span`/`Fixture`/`Policy`/`EvaluationResult` as JSON Schema, source of truth. | A trace created by one implementation is loaded and evaluated by another using only the published schema; round-trip tests pass in Go and one other language. |
| **2 — Extensible SDK + Wire Protocol** | Evaluator/Mutator/FixtureProvider registry, both tiers (§12, §15.4, §14.4); thin Python SDK. | Third-party evaluator/mutator addable in Go and in Python, without core changes, with identical result shapes. |
| **3 — Deterministic Evaluator Suite** | The 10 MVP evaluators (§13). | Synthetic-agent test matrix (wrong tool -> ToolSelection fail, etc.) passes; no paid AI service used. |
| **4 — Replay Engine** | Fixture Store, recording workflow, failure-injection modes (§14). | Zero live network calls during evaluation; determinism test (100 identical repeated runs) passes; each failure-injection mode verified. |
| **5 — Mutation Engine** *(the flagship phase)* | Mutators for the 9 MVP classes (§15.3); Detection Rate + False Positive Rate reporting. | ≥90% Detection Rate, ≤5% False Positive Rate, reproduced across repeated runs; `gust mutate` demo (§34) works end-to-end. |
| **6 — Policy Engine** | Policy schema, aggregation engine, verdict + evidence output (§16). | Same evaluator output through two different policy files yields two different correct verdicts. |
| **7 — CLI & CI** | `run`/`replay`/`evaluate`/`compare`/`mutate` as a single static binary; statistical significance gate (§20). | Repo can gate a PR in CI using the prebuilt binary; CI does not fail on statistically insignificant noise. |
| **7.5 — Trust Boundary** | Sandboxing for Tier 2 plugins (§26), before Phase 9 opens contrib adapters/evaluators to less-trusted code. | A misbehaving Tier 2 evaluator/mutator (infinite loop, network attempt, excess memory) is contained. |
| **8 — OTel/OpenInference Ingestion** | `ingest/otel/` adapter (§25); export evaluation/policy/mutation semantics back onto OTel infra. | An externally-instrumented (OTel/OpenInference) trace is ingested, evaluated, and the result is observable through standard OTel infrastructure. |
| **9 — Framework Adapters** | Generic adapter; one Python framework; one adapter with a different execution model/language (§5 P1.2). | Same evaluator suite produces comparable results across multiple frameworks and at least two languages. |
| **10 — Optional LLM-Based Evaluation** | `JudgeProvider` (Tier 1 + Tier 2), local/mock backends first. | A judge/rubric is marked "stable" only after clearing a stated correlation bar (Spearman ρ ≥ 0.7) against a ≥50-case human-labeled calibration set; otherwise ships "experimental." Model-based evaluation remains fully optional. |
| **11 — Statistical Regression Engine v2** | Bootstrap comparisons, effect size, minimum sample size, `INCONCLUSIVE` state — upgrade of Phase 7's gate. | Correctly distinguishes `A == B`, `A slightly > B`, `A significantly > B` on controlled synthetic samples. |
| **12 — Production Trace Evaluation** | Sampling/filtering policies feeding both evaluation and fixture recording; PII redaction by default. | Production evaluation runs without evaluating every request and without leaking unredacted PII by default. |
| **13 — Failure Mining** | Production failures -> clustering -> human-approved regression fixtures/dataset entries. | ≥80% of repeated synthetic failure patterns grouped correctly. |
| **14 — Multi-Agent Evaluation** | Handoff/delegation/coordination evaluators. | Known multi-agent mutations detected with the same Detection Rate/FPR discipline as §15. |
| **15 — AEE Public Benchmark** | Empirically measured, fairly run comparison against DeepEval/Phoenix/etc. (§24.1). | Numbers are published only once actually measured on real installations; methodology is documented and reproducible by a third party. |

---

# 30. Gold-Standard Criteria

**Technical:** stable schema; backwards compatibility (§27); language/framework neutrality enforced via the Tier 2 wire protocol, demonstrated in Phase 9; deterministic core; extensible evaluator/mutator/fixture API; OTel/OpenInference interoperability; reproducible experiments (content-addressed datasets and fixtures).

**Scientific:** mutation benchmark with a mandatory false-positive companion metric (§15.2); any future human-judge correlation with an explicit acceptance threshold (§29 Phase 10); regression detection accuracy; documented evaluator/policy methodology; reproducible benchmark results.

**Ecosystem:** multiple framework adapters in multiple languages; external evaluator/mutator plugins (sandboxed); external schema consumers; third-party contributions.

**Governance:** ADRs from Phase 0; RFC process once external contributors exist; Apache-2.0 license set from Phase 0; `spec/` evolves independently of the implementation, mirroring OpenTelemetry's own spec/SDK split.

**What we will not claim:** "gust will become the gold standard" is an ambition, not a technical requirement. The falsifiable claim is: *we will build the framework that can objectively demonstrate whether an evaluation system is reliable* — via §15's mutation methodology and §24's AEE metric. Whether the community treats the result as a gold standard is earned, not asserted.

---

# 31. What We Need to Prove

## H1 — Agent traces can be normalized across frameworks and languages
Verification: multiple framework adapters, across at least two languages, produce the same `AgentRun` schema.

## H2 — Agent failures can be detected independently of final output
Verification: mutation benchmark (§15) detects trajectory/tool/recovery failures even when final output is correct.

## H3 — Deterministic evaluation is sufficient for a useful MVP
Verification: MVP detects ≥90% of supported synthetic mutations with ≤5% false positives, without an LLM, replayed against recorded fixtures with zero live network calls.

## H4 — Evaluation can be made reproducible
Verification: same agent + same content-addressed dataset/fixture set + same evaluator versions produce identical results across repeated runs (§14.5).

## H5 — The abstraction is extensible across languages, not just across classes in one codebase
Verification: third-party evaluator, mutator, fixture provider, and (later) judge provider can be implemented in Go or via the wire protocol, without modifying core.

## H6 — A policy layer decoupled from evaluators produces meaningfully different, correct verdicts (new)
Verification: the same evaluator result set, run through two different policy configurations, yields two different and individually correct pass/fail verdicts (§16.5).

These six hypotheses are more important than building a large feature set.

---

# 32. Recommended MVP Technology Strategy

Unchanged from v0.2: **Go from Phase 0, no planned migration.**

## Why Go (unchanged reasoning, now additionally motivated by §23's speed claims)

1. **Distribution matches the CI-native use case** — single static binary, no dependency resolution, `curl`-and-run like `golangci-lint`/`hugo`/`gh`/the OTel Collector.
2. **Determinism-first fits a compiled, statically typed language** — the entire premise of §7.2/§7.3 (deterministic-first, replay-determinism) is best served by a language that catches structural bugs at compile time in code whose job is to be a trustworthy gate.
3. **Concurrency for the throughput claims in §23/§24** — "100,000 AgentRuns in single-digit seconds" is a goroutines-and-no-GIL problem, not a multiprocessing-workaround problem.
4. **Direct precedent** — Open Policy Agent (write a policy once, evaluate anywhere, in Go) is the closest structural analog, and its distribution story is part of why it won.
5. **No migration risk** — the domain model (§10) is a small, well-understood set of objects; design it once, carefully, in Go.

## Neutralizing the contribution-barrier cost

The Tier 2 wire protocol (§12) exists specifically so a Python-only contributor never needs to touch Go to add an evaluator, mutator, or fixture provider. This is designed in from Phase 2, not retrofitted.

## Stack

```text
Go 1.23+                          core engine, CLI, replay/mutation/policy engines, evaluator suite
JSON Schema (Draft 2020-12)       spec/ — source of truth, language-neutral
santhosh-tekuri/jsonschema        Go-side schema validation
Cobra                             CLI framework
gonum                             bootstrap/statistics (§20, Phase 11)
modernc.org/sqlite                pure-Go SQLite driver — only if persistence is needed
Go testing package + testify      unit/contract/integration tests
OpenTelemetry Go SDK              §25, §29 Phase 8
Python 3.12+ (sdk/python/)        thin SDK: AgentRun construction + Tier 2 protocol wrapper
```

Do not introduce Kubernetes, Kafka, Postgres, Redis, cloud infrastructure, or microservices during the MVP.

---

# 33. Cost-Constrained Development Plan

## Stage 1 — $0
Synthetic agents (Go) + recorded/synthetic fixtures + deterministic evaluators + mutation engine. **This is the entire MVP.** No live services, no LLM, no cloud.

## Stage 2 — $0 (P1)
Local model (Ollama/llama.cpp) reached over HTTP from a Go `JudgeProvider`, once Phase 10 begins — not before.

## Stage 3 — optional, later
Paid OpenAI/Anthropic/Gemini judge provider, as a plugin, only when useful, never required to validate the architecture.

You do not need a paid subscription, or even an LLM at all, to establish whether the architecture works — that is the entire point of removing the judge from the MVP (§6).

---

# 34. The MVP Killer Demo

This is the artifact the README leads with. It is the concrete acceptance test for Phases 3–6.

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
$ gust compare baseline.json candidate.json

                 baseline    candidate

Task success       94.2%       95.1%   +0.9%
Tool correctness   97.1%       96.8%   -0.3%
Recovery           91.4%       74.2%  -17.2%  <- REGRESSION
Latency            810ms       790ms   -2.5%

RESULT: FAIL (policy: default.yaml, clause: regression.recovery_drop)
```

Both commands must work **offline, with zero configuration beyond a local `agent.json` and dataset**, on a fresh clone of the repository, before Phase 6 is considered done.

---

# 35. Final Project Definition

### One-line description

> **gust is an open, language-neutral execution engine for replaying agents against recorded fixtures, mutation-testing evaluation suites for their own reliability, and gating regressions with a policy engine and structured evidence — not another metrics library.**

### Differentiation

```text
REPLAY-FIRST     — evaluation runs against a recorded, deterministic world, never live services
MUTATION-VALIDATED — the evaluator suite's own reliability is measured and published, not assumed
POLICY-DECIDES   — evaluators measure; a separate, declarative policy engine decides pass/fail
EVIDENCE-BASED   — structured expected/actual evidence, not a bare score
LANGUAGE-NEUTRAL — enforced by a wire protocol (§12), not asserted
CI-NATIVE        — single static binary, statistically rigorous from day one
FAST             — deterministic evaluation throughput as a stated, benchmarked number (§23, §24)
OTEL-COMPATIBLE  — an ingestion path, not a competing telemetry model
```

### What we are explicitly not trying to win

Metric count, evaluator breadth, or LLM-judge sophistication — DeepEval, Braintrust, and Phoenix already contest that ground well (§3). We are trying to win: *can you prove your evaluation suite actually catches broken agents, reproducibly, fast, in any language.*

### Long-term aspiration

> When someone builds an AI agent, gust is the layer they use to answer: did my agent actually work, how did it work, why did it fail, would we have caught it if it had failed differently, and can we prove that — regardless of what language their agent or their evaluator is written in.

---

# 36. Definition of Done — MVP (v0.3)

- [ ] `AgentRun`/`Span`/`Fixture`/`Policy`/`EvaluationResult` exist as JSON Schema (source of truth).
- [ ] Tier 1 (native Go) and Tier 2 (wire protocol) interfaces exist for evaluators, mutators, and fixture providers.
- [ ] Tier 2 reference implementation exists in two languages (Go, Python).
- [ ] The 10 MVP deterministic evaluators (§13) are implemented; **no LLM-based evaluator exists in the MVP**.
- [ ] Replay Engine works with zero live network calls during evaluation; all 5 failure-injection modes (§14.2) are implemented and tested.
- [ ] Determinism is proven: 100 repeated runs of the same `AgentRun` + fixture set produce identical results.
- [ ] Mutation Engine implements the 9 MVP mutation classes (§15.3) and reports Detection Rate + False Positive Rate together, always.
- [ ] ≥90% Detection Rate and ≤5% False Positive Rate achieved and reproduced across repeated runs.
- [ ] Policy Engine exists; the same evaluator output through two different policies yields two different, correct verdicts.
- [ ] Every failing built-in evaluator produces structured evidence (§17.3).
- [ ] Datasets and fixture sets are content-addressed.
- [ ] `gust run / replay / evaluate / compare / mutate` all work as a single static binary.
- [ ] CI integration includes a statistical significance gate from the first release, not just thresholds.
- [ ] The §34 killer demo runs end-to-end, offline, on a fresh clone.
- [ ] Untrusted Tier 2 plugins run sandboxed with resource limits.
- [ ] License (Apache-2.0) and `adr/` governance process exist from Phase 0.
- [ ] Documentation explains the four pillars (Replay/Evaluate/Mutate/Policy) and both extension tiers.
- [ ] The entire MVP runs without a paid AI API and without an LLM at all.

---

# 37. References

[1] OpenTelemetry — GenAI agent and framework semantic conventions. https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/gen-ai-agent-spans.md
[2] OpenTelemetry — GenAI semantic conventions. https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/gen-ai-spans.md
[3] DeepEval — agent evaluation quickstart and metrics (task completion, step efficiency, plan adherence, tool/argument correctness). https://deepeval.com/docs/getting-started-agents
[4] DeepEval — LLM tracing and metrics introduction. https://deepeval.com/docs/evaluation-llm-tracing , https://deepeval.com/docs/metrics-introduction
[5] Braintrust — systematic evaluation, datasets, experiments, CI regression. https://www.braintrust.dev/docs/evaluate
[6] Braintrust — best practices for evaluating agents (stubbing external dependencies). https://www.braintrust.dev/docs/best-practices/agents
[7] Arize Phoenix — tool-selection/tool-invocation evaluators, release notes. https://arize.com/docs/phoenix/release-notes/02-2026/02-01-2026-tool-selection-and-tool-invocation-evaluators
[8] Arize Phoenix — documentation home (OpenTelemetry integration, evaluators, datasets/experiments). https://arize.com/docs/phoenix/
[9] OpenTelemetry semantic conventions overview. https://opentelemetry.io/docs/specs/semconv/

---

# 38. Changelog: v0.2 → v0.3

| # | Area | Change |
|---|---|---|
| 1 | Thesis | Pivoted from "generic language-neutral evaluation framework" to "Replay + Mutate + Evaluate + Policy execution engine," positioned explicitly against DeepEval/Braintrust/Phoenix by name (§3). |
| 2 | Competitive analysis | Replaced generic gap-analysis table with named competitor capabilities and five specific undefended-territory claims (§3). |
| 3 | LLM judge | **Removed entirely from MVP** (was "optional Phase 7" in v0.2). Reserved as explicit P1 scope, gated behind the deterministic/replay/mutation stack being proven first (§6, §7.7, §29 Phase 10). |
| 4 | New core pillar | Replay Engine (§14) — recorded fixtures, deterministic world, 5 failure-injection modes. Previously absent entirely. |
| 5 | New core pillar | Mutation Engine (§15) elevated from "Phase 3 verification technique" to the project's primary, flagship, always-reported trust metric and first public demo. |
| 6 | New core pillar | Policy Engine (§16) — decouples "what evaluators measured" from "was this acceptable," replacing flat threshold-only CI config. |
| 7 | New principle | Evidence-over-scores (§7.6, §17) — `EvaluationResult` now requires structured evidence entries on failure, not just a numeric score. |
| 8 | North-star metric | Agent Evaluation Effectiveness (AEE, §24) composite metric introduced; aspirational public leaderboard vs. named competitors (P2, numbers-only-when-measured). |
| 9 | Repo/domain model | Added `replay/`, `mutate/`, `policy/`, `providers/fixture/`, `ingest/otel/`; added `Fixture`, `Mutator`, `MutationResult`, `Policy` to the core domain model (§9, §10). |
| 10 | CLI | Added `gust replay` and `gust mutate` as first-class commands; `mutate` is now the flagship demo command (§21, §34). |
| 11 | Phase roadmap | Reordered so Replay (Phase 4) and Mutation (Phase 5) come immediately after the deterministic evaluator suite and before CLI/CI, reflecting their new status as the primary validation method (§29). |
| 12 | Hypotheses | Added H6 (policy decoupling produces correct, distinguishable verdicts, §31). |
| 13 | Non-functional targets | Added throughput/latency targets tied to the killer demo's stated numbers (§23). |
| 14 | Everything else (schema-as-source-of-truth, Tier 1/2 wire protocol, Go-from-day-0, security/trust model, interface versioning, content-addressed datasets) | Carried forward from v0.2 largely unchanged in substance — these were sound infrastructure decisions independent of the thesis pivot. |
