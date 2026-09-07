# `gust` — Architect Review v2: Fix Verification + New Findings

**Reviewer role:** Software architect, 20+ years designing libraries/frameworks
**Subject:** `gust`, re-uploaded after the first review round (git log shows `9c09277 Fix Test-mode correctness bugs from architect review.` and related commits)
**This document supersedes `gust_Code_Review_and_Architecture_Report.md`.** It (1) verifies each of the 12 previously-reported findings against the actual diff, (2) checks the fixes themselves for new bugs, and (3) reports newly-discovered issues from files not covered in the first pass.

---

## 1. Verification of the Previous 12 Findings

All twelve were addressed. Verified against the actual code (not just the commit message) below — this matters because a fix that looks right in a diff summary sometimes isn't; each one was checked for correctness, not just presence.

| # | Finding | Status | Verification notes |
|---|---|---|---|
| 1.1 | Wilson z-value hardcoded lookup | **Fixed correctly** | `zForConfidence` now uses `math.Sqrt2 * math.Erfinv(confidence)`. Checked the math: for a two-sided interval, `z = Φ⁻¹((1+confidence)/2) = √2·erfinv(confidence)` — the substitution is exact, not an approximation, so this is now correct for *any* confidence in `(0,1)`, not just the three previously hardcoded values. Confidence `≤0` or `≥1` is now a returned error instead of a silent default. |
| 1.2 | Sampler discarded all N samples on one transient error | **Fixed correctly, and better than requested** | Per-sample failures are now tracked as `execErr` distinct from assertion failures, counted toward `ReliabilityResult.ExecutionErrors`, and a bounded single retry (`runWithRetry`, 50ms backoff) absorbs one-off blips before counting a sample as an execution error. A new `MaxExecutionErrorRate` (default 20%) aborts the scenario with a distinct `ErrRunnerUnstable` only when execution noise itself is excessive — exactly separating "the harness is unstable" from "the agent is unreliable," which the original request asked for. |
| 1.3 | Policy engine returned early, dropping later scenario results | **Fixed correctly** | The loop now sets a `hardFailed` flag and `continue`s instead of returning; `ScenarioResults`/`Violations` are fully populated for every input result before the final verdict is computed after the loop. |
| 1.4 | `on_flaky: warn` indistinguishable from `ignore` | **Fixed correctly** | `warn` now sets `OverallVerdict = VerdictFlaky` (exit code still `0`, correctly — it shouldn't block CI). `ignore` alone maps to `VerdictPass`. This is now honestly reported in the structured JSON, not just in the violations array. |
| 1.5 | Wire client `Close()` race → nil-pointer panic | **Fixed correctly** | `case resp, ok := <-respCh: if !ok { return ErrClientClosed }` — the exact fix recommended. |
| 1.6 | Unbounded wire-protocol line buffering | **Fixed correctly** | `readLineLimited` with a 16 MiB cap, backed by a 64 KiB `bufio.Reader`, correctly loops on `bufio.ErrBufferFull` and enforces the cap on every chunk. Logic checked for the loop-termination case (final chunk with `err == nil`) and the cap-exceeded case (`ErrLineTooLong`) — both correct. |
| 1.7 | No real resource sandboxing, docs overstated it | **Partially addressed** | A per-call timeout (`defaultEvaluateTimeout = 60s`) was added to `WireEvaluator.Evaluate`, which closes the "hangs forever" gap. **CPU/memory/network limiting is still not implemented** — this was flagged as the highest-effort, longest-term item last time and remains open. See §3 below: this is fine as a phased plan, but the documentation should still be explicit that today's trust boundary is *env scrubbing + a bounded per-call timeout*, not full resource isolation. |
| 1.8 | JCS canonicalization lost precision on large integers | **Fixed correctly** | Integer literals are now preserved exactly via `strconv.ParseInt`/`FormatInt` when they fit `int64`, with a documented, deliberate RFC 8785 deviation comment explaining why. `isIntegerLiteral` correctly handles the leading-minus-sign case and rejects anything with `.`/`e`/`E`, falling back to the float path as intended. |
| 1.9 | `SchemaValidationEvaluator` didn't validate output, only the envelope | **Fixed correctly** | Now uses `github.com/google/jsonschema-go/jsonschema` (added to `go.mod`), reads `expected.Parameters["schema"]`, parses `run.Outcome.Output` as JSON, and validates against the resolved schema. Falls back to envelope-only validation *with an explicit `envelope_only: true` evidence flag and message* when no schema is declared, so the weaker path is never silently mistaken for real output validation — a better fix than what was asked for. One thing worth double-checking deliberately: `sch.Resolve(nil)` is called specifically without a loader so a malicious `$ref` in a scenario-supplied schema can't trigger an outbound fetch — good instinct, and worth a one-line test asserting a `$ref: "http://..."` schema fails closed rather than attempting network access. |
| 1.10 | Scenario extractor mislabeled failed tool spans as "success" | **Fixed correctly** | Checks `span.Status.Code == "error"` and sets `Status: "error"`, `Error: span.Status.Message`, and a new `FailureModeRecordedError` mode distinct from the five synthetic injection modes — exactly the recommended distinction between "captured a real failure" and "authored an injected one." |
| 2.1 | Ordered-fixture sequence state unsafe under concurrent sampling | **Fixed correctly, and better than requested** | `MemoryFixtureProvider` now implements `Clone()` (deep-copies `exactFixtures`/`seqFixtures`, fresh `seqCounters`), and the sampler clones per-sample when the provider satisfies `ClonableFixtureProvider`. This is the better of the two options offered last time — it restores full parallelism instead of only falling back to `concurrency=1`. The `concurrency=1` fallback is correctly retained *only* for non-clonable providers with ordered fixtures. |
| 2.2 | `MaxStepsEvaluator` counted raw spans, not logical actions | **Fixed correctly** | Now counts only `SpanTypeTool` and `SpanTypeAgent` spans, excluding nested `llm`/`retrieval` sub-steps — matches the recommended definition. |
| 2.3 | Latency regression silently inert when baseline latency is 0 | **Not addressed** | `regression.go` is unchanged: `if baseline.LatencyNs > 0 { latRatio = ... }` still leaves `latRatio` at `0.0` when baseline latency is unset, which still reads as "no regression" rather than "not measurable." Low severity, still open — see backlog. |

**13 of 14 items (counting both original lists together) are fixed, several with a materially better solution than what was proposed rather than a minimal patch (§1.2, §1.9, §2.1 in particular).** This is a strong signal of engineering care, not just box-checking.

---

## 2. New Findings (not covered in the first review pass)

### 2.1 — `internal/core/dataset` (content-addressed dataset bundling) is fully implemented but completely unreachable from the CLI

**Files:** `internal/core/dataset/dataset.go`; confirmed via `grep -rln "gust/internal/core/dataset"` across the whole repository — **zero** other files import this package, and `internal/cli/root.go`'s command tree (`init`, `analyze`, `replay`, `test`, `mutate`, `compare`, `scenario`, `ingest`, `judge`) has no `dataset`/`bundle` subcommand.

**Impact:** The spec treats content-addressed, immutable datasets as a P0 concept (§15/§18/§20 — "a version+content mismatch is a hard error, not a silent overwrite"). The code to do this (`Bundle`, `Verify`) exists and is reasonably well-written, but **a user of the `gust` CLI cannot invoke it at all today.** This is a "the feature exists in the repo but not in the product" gap — worth resolving before any documentation or onboarding material implies dataset bundling is available, since right now it would be a dead link in practice.

**Fix:** Either wire it up (`gust dataset bundle <dir> --id <name>` / `gust dataset verify <dir>`) or, if it's intentionally deferred, move it under an explicit `experimental/` or `internal/wip/` path and note in `docs/` that dataset bundling is not yet CLI-accessible, so it doesn't read as a shipped capability.

### 2.2 — Even if wired up, `Bundle()` does not actually enforce the immutability it's named for

**File:** `internal/core/dataset/dataset.go`, lines 42–49

```go
path := filepath.Join(dir, sc.ID+".json")
data, err := json.MarshalIndent(sc, "", "  ")
...
if err := os.WriteFile(path, data, 0644); err != nil { ... }
```

**Impact:** `Bundle` writes every scenario file unconditionally, with no check against what's already on disk. Calling `Bundle` twice with the same `scenarioID` but different content **silently overwrites** the existing file — there is no hash comparison, no error, no `--force` requirement. This directly contradicts the stated design principle this package exists to implement. Relatedly, if a scenario is *removed* from the input slice on a re-bundle, its old `.json` file is never deleted, so a dataset directory can silently accumulate orphaned scenario files that `Verify()` won't even notice (it only checks IDs present in the current manifest).

**Fix:**
```go
if existing, err := os.ReadFile(path); err == nil {
    existingHash, _ := jcs.ContentHash(rawJSONToAny(existing))
    if existingHash != newHash {
        return nil, fmt.Errorf("scenario %s already exists with different content (want to overwrite? use --force)", sc.ID)
    }
}
```
plus a prune step that removes `*.json` files in `dir` whose scenario ID isn't present in the new manifest (behind the same `--force`, so accidental deletion isn't silent either).

### 2.3 — `Policy.Reliability.MaxExecutionErrorRate` has no explicit validation bounds

**File:** `pkg/api/types.go` (`PolicyReliability`) / `pkg/api/types.go` `Policy.Validate()`

The new `MaxExecutionErrorRate` field (added as part of the §1.2 fix) is read by the sampler with a `<= 0 → default 0.20` fallback, but `Policy.Validate()` doesn't check it's within `[0, 1]` the way `DefaultMinimumPassRate` is checked immediately above it. A policy author who sets `max_execution_error_rate: 5` (meaning "500%", presumably a typo for `0.5`) would pass validation and functionally disable the runner-instability guard entirely, silently. Low severity, but a two-line fix and consistent with the validation style already used for the sibling field two lines above it.

### 2.4 — `fs_store.go` uses the same unconditional-overwrite pattern in three places

**File:** `internal/adapters/storage/filesystem/fs_store.go`, lines 63, 138, 240 — all `os.WriteFile(filePath, data, 0644)` with no existence/hash check.

This is very likely **correct and intentional** for whatever this store is used for (a local results/run cache is expected to be mutable — re-running the same scenario ID should update its stored result, not error). Flagging it only so the team makes that an explicit, documented decision rather than an implicit one, and so nobody assumes `fs_store` provides the same immutability guarantee that `dataset.Bundle` is meant to (§2.2) — the two should not be confused as the same kind of storage.

---

## 3. Updated Backlog

**P0 — none remaining.** All P0 items from the first review are fixed and verified.

**P1:**
1. §2.1 — Wire `dataset bundle`/`dataset verify` into the CLI, or explicitly mark it experimental/unshipped in docs.
2. §2.2 — Make `Bundle()` refuse to silently overwrite differently-content scenario files; add orphan pruning behind an explicit flag.
3. §1.7 (carried over, unchanged from last review) — Real CPU/memory/network sandboxing for Tier‑2 plugins, or continue to be explicit in docs that only env-scrubbing + a call timeout exist today.

**P2:**
4. §2.3 — Validate `MaxExecutionErrorRate ∈ [0, 1]` in `Policy.Validate()`.
5. §2.3 (original numbering 2.3 from the first report) — Make "baseline latency not measurable" explicit in `RegressionResult` rather than silently reporting a `0.0` ratio.
6. §2.4 — Document that `fs_store` is an intentionally mutable local cache, distinct from `dataset.Bundle`'s immutability contract, so the distinction is a design decision on record rather than something a future contributor has to infer.

Yes. I reviewed the **latest `main` again**, and this is a meaningful jump from the previous version. I specifically looked at the new **AEE self-benchmark, 78-case Validation Suite, `gust init`, real runner path, fixture isolation changes, policy engine, and the updated architecture**. ([GitHub][1])

My overall conclusion:

> **Gust is now architecturally mature enough that I would stop adding broad framework features and spend the next phase proving correctness, tightening semantics, and validating the real-agent path.**

And I found **one important correctness bug** that I would fix before doing much else.

---

# 1. Overall assessment

I'd now rate it:

| Area                             |     Rating |
| -------------------------------- | ---------: |
| Core architecture                | **9.2/10** |
| Domain model                     |   **9/10** |
| Extensibility                    | **9.5/10** |
| CLI / usability                  | **8.8/10** |
| Testing discipline               |   **9/10** |
| Statistical design               | **8.8/10** |
| Correctness confidence           |   **8/10** |
| Adoption readiness               | **8.5/10** |
| Differentiation                  |   **9/10** |
| "Could become serious OSS infra" |   **9/10** |

The project now has a coherent story:

```text
Production / real run
        ↓
     AgentRun
        ↓
  Test Scenario
        ↓
 ┌──────┼────────┐
 ↓      ↓        ↓
Analyze Replay   Test
                ↓
             N samples
                ↓
          Wilson verdict
                ↓
             Policy
                ↓
               CI
```

And the new AEE/Validation layer adds:

```text
Gust
  ↓
test Gust itself
  ↓
mutation testing
validation suite
reproducibility
statistics verification
performance
```

That's a very good architecture for the problem you're trying to solve. 

---

# 2. The biggest improvement: you now have a way to prove Gust itself works

This was the biggest missing piece in the previous review.

You now have two distinct mechanisms.

### AEE

Your self-benchmark measures:

* mutation detection
* false positives
* evaluator throughput
* reproducibility
* Wilson classification

The latest run reports:

```text
Detection:       100%  (16/16)
FPR:             0%
Throughput:      8.4M cases/sec
Reproducibility: 100% (20/20)
H7:              all correct
```

and the CI gate is green. 

### Validation Suite

Much more interesting to me is the adversarial suite:

```text
78 cases
78 passed
0 failed

pass       18/18
fail       20/20
flaky       3/3
infra       4/4
mutation   18/18
fixture     9/9
recovery    6/6
```



That's exactly the direction I recommended.

And I looked at the actual cases, not just the report.

You're testing things like:

* wrong tool arguments
* forbidden tools
* missing tools
* exact vs subsequence trajectory matching
* duplicate tool calls
* malformed output
* missing schema fields
* Wilson boundary conditions
* infrastructure error rates
* ordered fixture exhaustion
* failure injection
* recovery after errors
* mutation detection

That's a **real validation catalog**, not just `assert(true)`. 

---

# 3. But be careful with the 100% claim

This is the first thing I'd change in the project's language.

Your report currently says:

> "When this gate is green, Gust correctly classifies the adversarial scenarios in this suite — evidence that the evaluator/statistics/fixture/mutation machinery can be trusted..."

The first half is absolutely defensible.

The second half is **too strong**.

Why?

Because the 78 expected outcomes are authored by you.

So this:

```text
78/78 correct
```

really proves:

> Gust correctly implements the expectations encoded in this validation suite.

It does **not** prove:

> Gust correctly evaluates arbitrary real-world agent behavior.

That's a classic test-oracle problem.

I'd change the claim to something like:

> **The validation suite provides regression evidence that Gust correctly handles a curated set of adversarial evaluation, fixture, mutation, recovery, statistical, and infrastructure scenarios.**

That is actually a very strong claim.

Don't oversell this. The people you're trying to attract are engineers, and they'll immediately understand the distinction.

---

# 4. Same issue with AEE

The AEE work is good, but I would slightly rename how you describe its metrics.

For example:

> **False positive rate: 0%**

is technically true for the golden suite.

But it isn't a general estimate of Gust's false-positive rate in the wild.

Similarly:

> **8.4M cases/sec**

is impressive, but the benchmark is measuring deterministic evaluator calls in-process, excluding LLM judges, rather than the throughput of an end-to-end `gust test` execution. The methodology explicitly says this. 

I'd label it:

> **Deterministic evaluator throughput: 8.4M evaluator calls/sec**

rather than:

> Eval throughput: 8.4M cases/sec

That distinction matters.

Otherwise someone will reasonably ask:

> "8.4 million agent tests per second?"

Obviously that's not what you're measuring.

---

# 5. I found an actual concurrency/fixture correctness issue

This is the most important thing I found.

You added `ClonableFixtureProvider`, which is good:

```go
Clone() ports.FixtureProvider
```

and the sampler detects it and clones the provider for each sample. 

Your `MemoryFixtureProvider` correctly implements cloning and resets sequence counters. 

**But the cloned provider isn't actually connected to the fixture proxy used by the agent.**

Look at the Mode 3 flow:

The CLI creates **one** provider:

```text
provider
   ↓
MockToolProxyServer(provider)
   ↓
fixtureEndpoint
```

and then passes that same endpoint to every runner. 

Inside the sampler, you do:

```text
provider := cfg.FixtureProvider

if clonable:
    provider = c.Clone()
```

but then the runner is still given:

```text
the same fixtureEndpoint
```

which points at the original proxy/provider. 

So:

```text
Sample 1 ─┐
Sample 2 ─┤
Sample 3 ─┼── same MockToolProxyServer
Sample 4 ─┘
              ↓
         original provider
              ↓
       shared seqCounters
```

The cloned provider is therefore not actually isolating the live agent's tool calls.

### Why this matters

You intentionally allow concurrency >1 when:

```text
HasOrderedFixtures && canClone
```

is true. 

And `MemoryFixtureProvider` **is clonable**.

So an ordered fixture scenario can run concurrently while still sharing the original fixture provider through the HTTP proxy.

That can produce:

```text
sample 1 → PENDING
sample 2 → DONE
sample 3 → fixture exhausted
```

instead of:

```text
sample 1 → PENDING → DONE
sample 2 → PENDING → DONE
sample 3 → PENDING → DONE
```

That's a **real correctness bug**, not a theoretical concern.

### I'd make this P0.

The architecture should become something like:

```text
                Sampler
                   │
        ┌──────────┼──────────┐
        ↓          ↓          ↓
    Sample 1    Sample 2    Sample 3
        │          │          │
   FixtureSession FixtureSession FixtureSession
        │          │          │
        ↓          ↓          ↓
      Agent      Agent       Agent
```

Each sample gets its own fixture world.

The easiest MVP solution may be **one ephemeral proxy per sample**.

Later you can optimize with session IDs:

```http
POST /v1/tools/call
X-Gust-Session: sample-17
```

and have one proxy multiplex isolated providers.

That would actually be a very good long-term architecture.

---

# 6. Your Validation Suite currently doesn't catch this

This is an important lesson from your own work.

You have a fixture test called:

> `fix_clone_isolates_sequence`

and it checks that cloning creates independent counters. 

That's good.

But you're testing:

```text
Clone()
```

not:

```text
Mode 3
+
concurrency
+
HTTP fixture proxy
+
ordered fixtures
+
multiple samples
```

So:

```text
unit correctness = PASS
system correctness = potentially FAIL
```

This is exactly why I'd now add a new Validation Suite category:

### `mode3`

Examples:

```text
mode3_ordered_fixture_concurrent
mode3_exact_fixture_concurrent
mode3_sample_isolation
mode3_fixture_exhaustion
mode3_retry_does_not_corrupt_fixture_state
mode3_otlp_trace_per_sample
```

This would be much more valuable than adding more evaluator types.

---

# 7. Another thing I don't completely like: automatic retry

Your sampler now does:

```text
runner.Run()
   ↓
error?
   ↓
wait 50ms
   ↓
runner.Run() again
```

for **any runner error**. 

I understand why you did this: transient network/provider blips shouldn't immediately count against the agent.

But this introduces a subtle problem.

Suppose the agent process genuinely crashes:

```text
sample 1
agent crashes
       ↓
retry
       ↓
agent succeeds
```

Gust now records a successful sample.

That may be what you want for infrastructure reliability, but it's not necessarily what you want for an agent test.

The more principled design is:

```text
Runner error
   ↓
classify error
   ├── transient infrastructure → retry
   ├── agent execution failure → count failure
   └── timeout/cancellation → policy-defined
```

Even better:

```text
RetryPolicy
  max_attempts
  retryable_errors
  backoff
```

I wouldn't necessarily implement all of this now.

But I would **stop calling the retry unconditional**.

At minimum, make it explicit in the scenario/policy.

---

# 8. Hard constraints are still more hard-coded than your model suggests

Your API has:

```go
HardConstraints {
    ForbiddenTools
    SchemaViolations
}
```

and your policy YAML exposes:

```yaml
hard_constraints:
  forbidden_tools: 0
  schema_violations: 0
```

But the policy engine effectively identifies hard failures by evaluator name:

```text
forbidden_tool
schema_validation
```

rather than using those configured thresholds. 

So right now the architecture says:

```text
policy configuration
       ↓
customizable
```

but implementation is closer to:

```text
forbidden tool → always hard
schema violation → always hard
```

That's okay **if that's the intended MVP invariant**.

If so, I'd simplify the public API.

If you want the policy fields, make them actually control the behavior.

I'd probably eventually move toward assertion-level criticality:

```yaml
assertions:
  - type: forbidden_tool_call
    criticality: hard

  - type: tool_sequence
    criticality: soft
```

You already have `CriticalityHard` / `CriticalitySoft` in the API, but the analyze engine currently doesn't use criticality when aggregating results. 

That's a sign that the model is getting ahead of the implementation.

Not P0, but I'd clean this up before adding more policy semantics.

---

# 9. The `gust init` addition is excellent

This is one of the changes I like most from an adoption perspective.

Now:

```bash
gust init
```

creates:

```text
gust.yaml
tests/
  policy.yaml
  scenarios/
    example.yaml
  fixtures/
  assertions/
```

and tells the user what to do next. 

That changes the product from:

> "Here's an evaluation engine. Figure out the concepts."

to:

> "Here's a testing tool. Start a project."

That's exactly what you want.

I would keep investing in this direction.

---

# 10. The README is now much better

The current opening is strong:

> Record a real agent run. Turn it into a reproducible scenario. Inject failures. Run the agent repeatedly. Catch behavioral regressions in CI.

That's basically the whole product in one sentence. 

And the Mode 1/2/3 distinction is immediately visible.

I also like that you now explicitly tell people:

```text
synthetic → no GPU required
ollama → local real model
http → real external agent
exec → real local agent
```

That makes the project accessible to someone without an expensive AI subscription.

The README now makes the progression much clearer. 

---

# 11. Your architecture is now surprisingly extensible

This remains one of Gust's biggest strengths.

You have explicit extension points for:

```text
Evaluator
Mutator
TestRunner
JudgeProvider
FixtureProvider
RunStore
```

and cross-language JSON-RPC plugins. 

The important thing is that the built-ins themselves use the same interfaces that external implementations use.

That's the correct architecture.

You're not creating:

```text
Gust core
+
special extension hacks
```

You're creating:

```text
ports
  ↑
built-in adapters
  ↑
external adapters
```

That's exactly how I'd want an infrastructure project designed for a future ecosystem.

---

# 12. I especially like the decision around LLM judges

The new design still keeps the LLM judge optional, with `allow_llm_judge` defaulting to false. 

That's important.

I wouldn't allow the project to drift toward:

> "Gust evaluates your agent by asking another LLM whether it did well."

The deterministic foundation is your differentiator.

LLM judges should be:

```text
soft signal
+
optional
+
calibrated
+
versioned
```

not the foundation of CI.

You're currently preserving that distinction.

---

# 13. One architectural improvement I'd now make: separate "test infrastructure" from "evaluation semantics"

You are very close to this already.

I would explicitly think of Gust as three layers:

```text
┌─────────────────────────────────────┐
│          Evaluation semantics       │
│                                     │
│ task success / tool / schema / etc. │
└──────────────────┬──────────────────┘
                   │
┌──────────────────▼──────────────────┐
│          Agent test engine          │
│                                     │
│ scenario / fixtures / replay /      │
│ mutation / sampling / statistics    │
└──────────────────┬──────────────────┘
                   │
┌──────────────────▼──────────────────┐
│          Execution adapters         │
│                                     │
│ HTTP / exec / Ollama / OTel / SDK   │
└─────────────────────────────────────┘
```

Your code is already trending this way.

That is good because eventually somebody may say:

> "I don't want Gust's built-in evaluator. I want my own domain evaluator."

and the answer should be:

> "Fine. The test engine doesn't care."

Your extension architecture already supports this. 

---

# 14. The self-benchmark is now good enough to become a project feature

I wouldn't hide AEE/Validation in internal docs anymore.

This is actually part of your story:

> **Gust tests itself.**

That's compelling.

But I'd phrase it carefully:

### Instead of

> Gust is 100% correct.

### Say

> Gust maintains an adversarial validation suite and mutation benchmark that continuously regression-tests its own evaluation machinery.

That's a **great open-source engineering story**.

And the fact that the benchmark is CI-generated and reproducible is good infrastructure practice. 

---

# 15. What I think Gust should NOT do now

This is probably the most important advice.

**Stop adding surface area.**

You already have:

```text
Analyze
Replay
Test
Mutation
Statistics
Policy
Scenario extraction
Fixtures
Failure injection
OTel
Langfuse
Python
TypeScript
Go
JSON-RPC
LLM judge
HTTP runner
Exec runner
Ollama
AEE
Validation Suite
CLI scaffolding
CI integration
```

That's enough.

Your README now describes Phases 10–12 and 17 as landed, with future work including stats v2, continuous evaluation, clustering, multi-agent and real-agent E2E. 

**I would resist all of those for now.**

---

# 16. The next phase I'd make very small

I'd call it:

## Gust Trust Phase

### P0

**1. Fix fixture isolation**

This is the one I would fix immediately.

```text
sample
  ↓
isolated fixture session
  ↓
agent
```

### P0

**2. Add Mode-3 concurrency validation**

Especially:

```text
ordered fixture
+
4 workers
+
20 samples
```

and assert every sample gets the same deterministic sequence.

### P0

**3. Make retry semantics explicit**

Don't blindly retry every runner error.

### P1

**4. Make policy semantics match the public schema**

Either:

* actually implement `HardConstraints`, or
* remove/de-emphasize the configuration until it is implemented.

### P1

**5. Use assertion criticality**

You already have the model. Finish the semantic connection.

### P1

**6. Add a real-agent end-to-end test**

This is now the most important missing proof.

Something like:

```text
local agent
   ↓
real LLM
   ↓
Gust fixture proxy
   ↓
real tool calls
   ↓
OTel trace
   ↓
Gust evaluator
   ↓
Wilson
   ↓
CI verdict
```

You don't need OpenAI/Anthropic API spending to make the framework useful here.

**Ollama + a small local model is enough for the infrastructure E2E.**

And for the deterministic framework tests, you don't need an LLM at all.

---

# 17. The really big missing proof

There's one thing I'd like to see before I would call Gust **production-quality testing infrastructure**.

Not:

```text
78 synthetic validation cases
```

You've got that.

Not:

```text
16/16 mutation detection
```

You've got that too.

I want:

```text
                REAL AGENT
                    │
                    ▼
             REAL LLM MODEL
                    │
                    ▼
             REAL TOOL CALL
                    │
              Gust proxy
                    │
           controlled fixture
                    │
                    ▼
              REAL TRACE
                    │
                    ▼
                 GUST
                    │
             20-100 runs
                    │
                    ▼
             reliability verdict
```

Then deliberately change the agent:

```text
Version A
  ↓
97% pass

Version B
  ↓
82% pass
```

and demonstrate:

```text
CI → REGRESSION
```

Then inject:

```text
timeout
malformed result
wrong tool
wrong argument
stale data
```

and show Gust detecting each.

**That would be the demo that convinces me.**

---

# Final verdict

I am **more bullish on Gust now than after the previous review**.

The project has crossed an important architectural threshold.

It's no longer just:

> "I built an agent evaluation framework."

It is becoming:

> **"I built a testing system for probabilistic software, with controlled environments, replay, fault injection, statistical reliability, regression gates, and a mechanism for testing the testing system itself."**

That's a much stronger engineering project.

The most impressive change isn't another evaluator or another integration.

It's this:

```text
Gust
 ↓
tests agents
 ↓
but Gust itself is tested
 ↓
mutation benchmark
 ↓
adversarial validation
 ↓
reproducibility
 ↓
statistical verification
```

That's the direction I'd continue.

### My priority order now

```text
                    NOW
                     │
                     ▼
           Fix fixture isolation
                     │
                     ▼
          Validate concurrent Mode 3
                     │
                     ▼
          Harden retry/policy semantics
                     │
                     ▼
         Real local-LLM end-to-end test
                     │
                     ▼
          Publish benchmark methodology
                     │
                     ▼
              THEN add features
```

And importantly, **I would not redesign Gust after this review**. The core architecture is good. I'd harden the contracts around it.

The one issue I'd immediately hand to your coding agent is the **fixture-session isolation bug**, because that's the kind of subtle concurrency issue that can make a testing framework produce a misleading green CI result. The fact that the new Validation Suite didn't catch it is itself useful feedback: **your next evolution should test system-level invariants, not only individual component invariants.**

[1]: https://github.com/ShadyD45/gust "GitHub - ShadyD45/gust: Behavioral testing infrastructure for AI agents. Reproduce runs, inject failures, and catch regressions · GitHub"


---

## 4. Overall Assessment

This is now a genuinely solid MVP implementation of the spec. The fix pass didn't just patch symptoms — in three cases (§1.2, §1.9, §2.1 above) the team's fix is architecturally better than the minimal version I proposed (retry + distinct execution-error accounting instead of just "don't abort"; real JSON Schema validation with a hardened `$ref` policy instead of a bare validator call; per-sample provider cloning that preserves full parallelism instead of just falling back to serial execution). The remaining open items are lower-severity and none of them sit on the statistics/policy critical path the way the original P0 issues did. The one adoption-relevant gap worth prioritizing next is §2.1/§2.2 — the dataset-bundling feature existing in the codebase but not being reachable from the CLI is the kind of thing that quietly undermines trust once a user notices the spec promises something the tool doesn't yet expose.

