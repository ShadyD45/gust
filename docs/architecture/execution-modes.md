---
title: Execution modes
nav_order: 2
parent: Architecture
has_mermaid: true
---
# Execution Modes

gust never blurs Analyze, Replay, and Test. Each answers a different question.

```mermaid
flowchart LR
  accTitle: gust execution modes
  accDescr: Analyze evaluates a captured run, Replay rewrites a run from fixtures, and Test samples a live agent into a Wilson verdict

  subgraph m1 [Mode 1 — Analyze]
    direction TB
    aIn[AgentRun.json] --> analyze[Analyze]
    asserts[TestScenario assertions] --> analyze
    analyze --> evidence[Evaluation evidence]
  end

  subgraph m2 [Mode 2 — Replay]
    direction TB
    rIn[AgentRun + Fixtures] --> replay[Replay]
    replay --> rewritten[Deterministic rewritten run]
  end

  subgraph m3 [Mode 3 — Test]
    direction TB
    tIn[Live agent + Fixtures] --> test[Test]
    test --> samples[N samples]
    samples --> verdict[Wilson verdict]
  end
```

## Mode 1 — Analyze

**Question:** Did this captured trajectory satisfy the assertions?

- Input: `AgentRun` + assertions
- Process: Dispatch each assertion to a registered deterministic evaluator
- Output: Per-assertion evidence and aggregate pass/fail
- Side effects: None (offline, no LLM)

## Mode 2 — Replay

**Question:** What happens if we re-apply recorded tool responses (and optional failure modes) to this trajectory structure?

- Input: `AgentRun` + fixture provider
- Process: Replace tool span outputs from fixtures; support sequence and hash matching
- Output: Deterministic rewritten run (stable under repeated replay)
- Side effects: Local fixture lookup only

Replay is also the substrate for **mutation testing**: mutators alter traces; Analyze detects whether assertions fail.

## Mode 3 — Test

**Question:** Is *today’s* live agent reliable on this scenario?

- Input: `TestScenario` + `TestRunner` + fixture endpoint
- Process: Run the agent *N* times in parallel (bounded workers); evaluate each sample; compute Wilson interval
- Output: `ReliabilityResult` with `PASS` / `FAIL` / `FLAKY` / `INSUFFICIENT_SAMPLES`
- Side effects: Invokes real agent/LLM (or a synthetic runner for statistical proofs)

Replay rewrites a **recorded** trajectory against fixtures. It does not start your agent. Injecting a timeout into Replay shows what the *trace* would look like with that response — to see whether the *agent* recovers, you re-run it under Test with the same fixtures (Mode 3).

## Testing pyramid (Replay → Simulated → Live)

```text
                    TEST
                      │
          ┌───────────┼────────────┐
          │           │            │
          ▼           ▼            ▼
       Replay      Simulated     Live
          │           │            │
       ~free        cheap        realistic
          │           │            │
      deterministic controlled   stochastic
```

A practical split:

| When | What | Why |
|------|------|-----|
| PR | Analyze + Replay + a small synthetic or fixture-backed Test | Fast, no paid tokens required |
| Nightly | Live `gust test` against a QA/Ollama agent, more samples | Measures real trajectories |
| Production | Sampled traces → `scenario from-run` → human assertions → CI | A behavioral bug becomes a regression test |

This is **reproducible testing for probabilistic agents**: the environment, fixtures, failure injection, assertions, and statistics are controlled; the agent is allowed to wander.

In-tree proof of that split: the [live-agent demo]({% link usage/live-agent-demo.md %}) (`llama3.2:3b` N=20 Wilson gate, scripted fallback). Replay/synthetic stay on `./demo/run.sh`.

## Policy layer

Policy sits **above** raw evaluation. It combines named hard-constraint **counts** (`forbidden_tools` / `schema_violations` maxima), Wilson reliability thresholds, `on_flaky` behavior, and optional baseline-vs-candidate regression into CI exit codes. See [Tuning the gate]({% link usage/tuning.md %}).

