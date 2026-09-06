# gust: Comprehensive Master Phased Roadmap

## 1. Vision & Strategy

gust is **test infrastructure for autonomous software** — the equivalent of JUnit, Mockito, and CI regression gating for LLM-driven agents. Because LLMs are non-deterministic, gust treats correctness as a **statistical pass rate** measured across repeated samples with confidence intervals, rather than a brittle single-run boolean.

This master roadmap outlines the progression of the project from inception (Phase 0) through the production-ready MVP (Phases 1–9) and forward into advanced ecosystem capabilities (Phases 10–18).

---

## 2. Roadmap Overview

```text
+---------------------------------------------------------------------------------------+
| PHASE 0: Research, Governance (ADRs, Apache-2.0)                                     |
+-------------------------------------------+-------------------------------------------+
                                            |
                                            v
+---------------------------------------------------------------------------------------+
| MVP FOUNDATION (Phases 1 - 5)                                                         |
| - Phase 1: Core Domain Entities, Schemas (Draft 2020-12), RFC 8785 Canonical JCS      |
| - Phase 2: Native Go Interfaces (Tier 1) & Cross-Language Wire Protocol (Tier 2)     |
| - Phase 3: Deterministic Evaluator Suite (10 MVP Evaluators with Structured Evidence) |
| - Phase 4: Mode 1 (Analyze) & Mode 2 (Replay) + Stateful Fixture Store & Mock Proxy  |
| - Phase 5: Mutation Testing Engine (9 Mutation Classes, Replay Mode, Detect/FPR)     |
+-------------------------------------------+-------------------------------------------+
                                            |
                                            v
+---------------------------------------------------------------------------------------+
| MVP FLAGSHIP & TEST MODE (Phases 6 - 9)                                               |
| - Phase 6: Mode 3 (Test) + Statistical Reliability Engine (Wilson Score, Ollama/Local)|
| - Phase 7: Policy Engine (Hard/Soft Constraints, FLAKY handling, Regression Compare)  |
| - Phase 8: Assisted Scenario Extraction (gust scenario from-run, Invariants)     |
| - Phase 9: Single Static Binary CLI, CI Pipeline Integration, & Killer Demo          |
+-------------------------------------------+-------------------------------------------+
                                            |
                                            v
+---------------------------------------------------------------------------------------+
| POST-MVP & ECOSYSTEM SCALE (Phases 10 - 18)                                           |
| - Phase 10: OpenTelemetry / OpenInference Ingestion & Semantic Conventions            |
| - Phase 11: Framework Adapters (LangChain, AutoGen, CrewAI, Python/TypeScript SDKs)   |
| - Phase 12: Optional Calibrated LLM Judge (Deferred until Spearman rho >= 0.7)        |
| - Phase 13: Statistical Regression Engine v2 (Bootstrap CI, Effect Sizes)             |
| - Phase 14: Production Continuous Evaluation & Automated PII-Redacted Mining          |
| - Phase 15: Failure Clustering & Deduplication Engine                                 |
| - Phase 16: Multi-Agent Coordination Evaluators & AEE Self-Benchmark                  |
| - Phase 17: Semantic Hardening & Adoption Front Door (evaluators, init, mutation UX)  |
| - Phase 18: Real-Agent End-to-End Example (deferred; builds on LangChain demo)        |
+---------------------------------------------------------------------------------------+
```

---

## 3. Detailed Phase Breakdown

### Phase 0: Research & Governance (Completed)
- **Scope**: Finalize the domain model and ADRs, create open source licensing (Apache-2.0).
- **Deliverables**: Governance docs, license, initial domain contracts in `spec/schemas/`.

### Phase 1: Core Domain Entities & Canonical Serialization
- **Scope**: Define JSON Schemas (Draft 2020-12) for `AgentRun`, `Span`, `Fixture`, `TestScenario`, `Assertion`, `Policy`, `EvaluationResult`, and `ReliabilityResult`. Implement RFC 8785 JSON Canonicalization Scheme (JCS) with SHA-256 for deterministic content addressing.
- **Go Best Practice**: Strongly typed domain models, zero external schema dependencies at runtime, strict JSON unmarshaling, builder patterns.

### Phase 2: Extensible Interfaces & Cross-Language Wire Protocol
- **Scope**: Define Tier 1 native Go interfaces (`Evaluator`, `Mutator`, `FixtureProvider`, `TestRunner`) using `context.Context`. Implement Tier 2 JSON-RPC 2.0 stdio protocol for Python/TS plugins with process isolation and timeout enforcement.
- **Go Best Practice**: Interface Segregation Principle (small, single-method or focused interfaces), pluggable registry pattern with thread-safe sync maps.

### Phase 3: Deterministic Evaluator Suite
- **Scope**: Implement the 10 MVP deterministic evaluators (`TaskSuccess`, `ToolSelection`, `ToolArguments`, `ToolSequence`, `ForbiddenTool`, `RequiredTool`, `MaxSteps`, `MaxLatency`, `ErrorRecovery`, `SchemaValidation`).
- **Target**: Zero network calls, zero cost, $\ge 1,000$ evaluations/second per core, rich structured evidence.

### Phase 4: Fixture Store, Analyze & Replay Engines
- **Scope**: Build content-addressed Fixture Store. Support both hash-based matching and stateful sequential FIFO matching. Build ephemeral Tool Mock Proxy Server (HTTP/JSON-RPC/MCP) for tool interception. Implement Mode 1 (Analyze) and Mode 2 (Replay) with 5 failure-injection modes (`success`, `timeout`, `malformed`, `slow`, `partial_failure`).
- **Exit Criteria**: 100 repeated Replay runs produce bit-for-bit identical results with zero external network traffic.

### Phase 5: Mutation Testing Engine (Replay Mode)
- **Scope**: Implement 9 mutation classes. Build Replay-mode mutation runner. Track typed mutation outcomes (`Applied`, `Skipped`, `Error`). Measure Detection Rate ($\ge 90\%$) and False Positive Rate ($\le 5\%$) against a golden test suite.
- **Exit Criteria**: Evaluator suite's own reliability is mathematically validated.

### Phase 6: Mode 3 (Test) & Statistical Reliability Engine
- **Scope**: Implement `TestRunner` for live agents. Support local LLMs (Ollama) for zero-cost testing. Build parallel sampling execution engine with worker pools. Implement Wilson Score confidence interval engine with boundary clamping and zero-variance handling.
- **Exit Criteria**: Correctly classify 99% reliable synthetic agent as `PASS` and 85% flaky agent as `FLAKY` against a 95% threshold over 20 samples (Hypothesis H7).

### Phase 7: Policy Engine & Regression Comparator
- **Scope**: Evaluate hierarchical policy rules: hard security constraints (forbidden tools, schema violations) and soft reliability thresholds. Implement `on_flaky` actions (`warn`, `fail`, `ignore`). Implement baseline vs candidate regression comparator.
- **Exit Criteria**: Same test run produces different correct CI outcomes under varying `on_flaky` configurations.

### Phase 8: Assisted Scenario Extraction (`scenario from-run`)
- **Scope**: CLI workflow converting production `AgentRun` traces into reusable `TestScenario` files. Automatically extract input and candidate fixtures.
- **Invariants**: Structurally prevent auto-populating assertions from observed agent behavior to prevent enshrining bugs (Hypothesis H8).

### Phase 9: CLI, CI Integration & Golden Demo
- **Scope**: Build single static binary using Cobra. Implement formatted outputs (human CLI table, CI summary, machine-readable JSON). Implement CI exit codes. Validate the complete end-to-end killer demo.
- **Exit Criteria**: Complete MVP Definition of Done satisfied; reproducible on fresh clone.

---

## 4. Post-MVP Roadmap (Phases 10 - 18)

Detailed engineering plans: **[`docs/plans/framework/`](framework/README.md)**.

| Phase | Title | Description | Target Milestones |
|---|---|---|---|
| **10** | OTel / OpenInference Ingestion | Ingest traces directly from OpenTelemetry Collector and OpenInference standards into Analyze mode. | OTel receiver, semantic mapping |
| **11** | Framework Adapters & SDKs | Publish lightweight client SDKs for Python and TypeScript; adapters for LangChain, LlamaIndex, AutoGen, CrewAI. | PyPI / npm packages, agent decorators |
| **12** | Optional Calibrated LLM Judge | Introduce `JudgeProvider` as an optional plugin; strictly gated by Spearman $\rho \ge 0.7$ on 50+ calibration cases. | Local judge (Llama 3), human alignment |
| **13** | Statistical Regression Engine v2 | Upgrade to bootstrap confidence intervals, Cohen's $d$ effect sizes, and dynamic sample size recommendation. | Non-parametric distributions, power analysis |
| **14** | Production Continuous Eval | Sample live production traces with default PII redaction and automated `TestScenario` proposal generation. | Streaming sampler, differential privacy |
| **15** | Failure Mining & Clustering | Group similar production failures using semantic clustering to eliminate duplicate test scenario proposals. | Vector/DBSCAN clustering, deduplication |
| **16** | Multi-Agent & Self-AEE | Multi-agent handoff/coordination evaluators; AEE remains a gust self-benchmark (no peer-framework leaderboard). | Multi-agent suite, CI self-score |
| **17** | Semantic Hardening & Adoption | Correct evaluator/validation semantics; `gust init`; mutation score prominence; README front door. | Match/occurrence/recovery/latency/Validate; init CLI |
| **18** | Real-Agent E2E Example | Realistic demo with real LLM + tools + intentional failure Gust detects (extends LangChain demo). | Deferred until after Phase 17 |
