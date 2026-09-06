I reviewed the current public repository, README, architecture docs, specification, SDK surface, core engines, evaluator implementation, and test-runner design. I also tried to run the repository locally, but the execution environment could not reach GitHub, so I’m **not claiming I ran the project’s test suite**. The assessment below is based on the code and documentation currently published in the repo. ([GitHub][1])

## My overall verdict

**Gust is substantially better than I expected at this stage.**

I would currently put it around:

| Area                           |     Rating |
| ------------------------------ | ---------: |
| Problem / thesis               |   **9/10** |
| Architecture                   | **8.5/10** |
| Extensibility                  |   **9/10** |
| Developer ergonomics           | **7.5/10** |
| Correctness of core semantics  |   **7/10** |
| Adoption readiness             | **7.5/10** |
| Differentiation                | **8.5/10** |
| Potential as serious OSS infra |   **9/10** |

The important thing is that **I would keep building this**. I would *not* redesign it.

The architecture has the right shape. What you need now is less “more features” and more **semantic hardening + making the first 10 minutes dramatically easier**.

---

# 1. The architecture is genuinely strong

The biggest architectural win is the separation of:

```text
AgentRun
   ↓
Analyze
   ↓
Replay
   ↓
Test
```

and specifically your rule that:

> a trace is evidence, not the test.

That is exactly the right abstraction boundary.

Your `TestScenario` contains the task, controlled environment, assertions, reliability configuration and provenance, while `AgentRun` represents an observed execution. ([GitHub][2])

That prevents the classic mistake of doing:

```text
production trace
      ↓
"expected behavior"
```

which can accidentally encode the bug into the test.

I also really like that the specification explicitly treats **live Test mode as the only mode that answers whether today's agent still works**. ([GitHub][3])

That's an architectural decision worth protecting.

### Another strong decision: protocol first

You have:

```text
pkg/api
spec/schemas
internal/ports
internal/core
internal/adapters
SDKs
```

rather than embedding everything into one CLI implementation. ([GitHub][1])

That's exactly what gives Gust a chance of becoming infrastructure rather than a personal CLI.

Your public `AgentRun`, `Fixture`, `TestScenario`, `Assertion`, `Policy`, and `ReliabilityResult` types also form a pretty coherent domain model. 

---

# 2. The OTel decision is excellent for adoption

This is one of the things I'd keep exactly as-is.

You aren't saying:

> “Throw away your observability stack and adopt Gust.”

Instead:

```text
Agent
  ↓
existing OTel/OpenInference instrumentation
  ↓
Gust
  ↓
test / analyze
```

The repo explicitly supports OTLP HTTP/gRPC ingestion and maps OpenInference-style attributes into the Gust model. ([GitHub][4])

That is an **extremely important adoption strategy**.

Someone using Phoenix, LangSmith's OTel exporter, OpenLLMetry, a collector, etc. doesn't necessarily have to instrument another framework just to try Gust. ([GitHub][4])

Your statement:

> “integration is a recorder in your tool-dispatch path — not a rewrite”

is exactly the right adoption message. ([GitHub][5])

---

# 3. The Go core is also making sense

At first I wasn't convinced Go was important here.

After looking at the implementation, I am.

You're using Go where it gives you value:

```text
deterministic evaluation
scenario processing
replay
mutation
statistics
policy
CLI
protocol
```

while allowing Python/TypeScript agents to remain Python/TypeScript.

The Python SDK being essentially dependency-free is a particularly good choice because agent projects already have dependency explosions. ([GitHub][6])

The TypeScript SDK is also a good signal that you aren't accidentally designing a Go-centric protocol. ([GitHub][7])

---

# 4. The statistics model is much better than "run it 5 times"

This is another area where I think the project is intellectually stronger than many agent-eval projects.

You're not simply doing:

```text
9/10 = PASS
```

You're using Wilson intervals and distinguishing:

```text
PASS
FAIL
FLAKY
INSUFFICIENT_SAMPLES
```

with hard constraints separated from probabilistic reliability. ([GitHub][8])

The specification even catches the subtle sample-size issue where 20/20 cannot establish a 95% lower bound at 95% confidence. That's the kind of detail that makes me more comfortable calling this an engineering project rather than an LLM demo. ([GitHub][8])

---

# 5. Mutation testing is probably your strongest differentiator

I still think this is the part I'd push much harder publicly.

Your architecture treats mutation testing as a way to measure whether the **evaluation system itself is trustworthy**, rather than pretending that passing assertions means the evaluators are good. ([GitHub][3])

That's excellent.

Conceptually:

```text
             Agent
               ↓
            AgentRun
               ↓
       ┌──────────────┐
       │   Evaluators │
       └──────┬───────┘
              ↓
          PASS / FAIL

Mutation
    ↓
broken AgentRun
    ↓
should evaluators detect it?
```

That is much more compelling than:

> “We have 12 more evaluation metrics.”

I'd actually make this one of the first things someone sees on the homepage.

---

# 6. Where I found real correctness concerns

This is where I'd be somewhat strict before calling Gust production-grade.

## A. `tool_sequence` currently means "subsequence", not sequence

Your evaluator explicitly allows extra tool calls between expected calls:

```text
expected:
A → B → C

actual:
A → X → B → Y → C
```

passes.

The code is doing subsequence matching rather than exact trajectory matching. ([GitHub][9])

That's not necessarily wrong, but the name `tool_sequence` strongly implies exact order.

I'd change the assertion semantics to explicitly support:

```yaml
type: tool_sequence
parameters:
  sequence: [A, B, C]
  match: subsequence
```

and:

```yaml
match: exact
```

Potentially later:

```yaml
match: ordered
allow_extra: true
```

This is a small API change with a large correctness benefit.

---

## B. `tool_arguments` can pass the wrong invocation

Currently, the evaluator loops through **all matching tool spans** and returns success as soon as any invocation has matching arguments. ([GitHub][9])

Example:

```text
1. cancel_order(999)   ← wrong
2. cancel_order(123)   ← correct
```

An assertion:

```yaml
tool: cancel_order
arguments:
  order_id: 123
```

will pass.

For many tests, that's not what you want.

You should eventually support explicit occurrence semantics:

```yaml
occurrence: first
occurrence: last
occurrence: any
occurrence: 2
```

or perhaps:

```yaml
tool_call:
  tool: cancel_order
  occurrence: 1
  arguments:
    order_id: 123
```

This becomes especially important for agents that retry.

---

## C. `error_recovery` is currently too coarse

This one is more important.

The implementation effectively reasons:

```text
error occurred
+
there is another step
+
overall outcome = completed
=
recovered
```

The current implementation checks for an error span and then treats a later step plus a completed outcome as recovery. ([GitHub][9])

Consider:

```text
get_customer()
   ↓
ERROR

retrieve_weather()
   ↓
SUCCESS

final_answer()
```

Gust could interpret this as recovery even though the original error was never actually recovered from.

The evaluator should eventually reason about **causal recovery**, not just temporal recovery.

For example:

```yaml
error_recovery:
  after_error:
    tool: get_customer
  recovery:
    required:
      - retry:get_customer
      - successful:get_customer
```

Or more generically:

```text
error span
   ↓
same operation / dependent operation
   ↓
successful continuation
```

This is an area I'd put on the correctness roadmap.

---

## D. Max latency assumes trace ordering

`MaxLatencyEvaluator` calculates:

```text
last span end - first span start
```

rather than deriving the actual earliest start and latest end. ([GitHub][9])

That assumes the trace slice is already ordered.

OTel generally gives you enough metadata to reason about spans, but a trace representation shouldn't depend on exporter ordering unless the contract explicitly guarantees it.

I'd calculate:

```text
min(start_time)
max(end_time)
```

across the relevant trace.

Even better, make the latency source explicit:

```text
wall_clock
agent_duration
tool_duration
llm_duration
```

because these are very different concepts.

---

## E. `AgentRun.Validate()` is weaker than the documentation implies

This caught my attention.

The documentation says `AgentRun.Validate` enforces the field rules, including span type. ([GitHub][5])

But the implementation checks required IDs/names and outcome status, while it does **not** validate that every `Span.Type` is one of:

```text
agent
llm
tool
retrieval
memory
plan
error
```

and it doesn't validate some of the other structural invariants described by the docs. 

That's dangerous because your whole evaluator architecture depends heavily on conventions such as:

```text
type == tool
```

For infrastructure, I would make the domain invariant stronger rather than relying on documentation.

---

# 7. The biggest adoption problem isn't architecture

It's this:

> **You still make users understand too much before getting value.**

Your documented path is good:

```text
record AgentRun
        ↓
declare assertions
        ↓
analyze
        ↓
extract scenario
        ↓
test repeatedly
```

But that's still a lot for someone discovering Gust for the first time. ([GitHub][5])

The person landing on the repo should ideally think:

> “I can try this in 2 minutes.”

Not:

> “Interesting. I need to understand AgentRun, scenarios, fixtures, assertions, policies and runners.”

The current killer demo helps significantly here. ([GitHub][1])

But I would go one step further.

### Your first experience should be something like

```bash
gust init
```

producing:

```text
gust.yaml
tests/
  scenarios/
  fixtures/
```

and then:

```bash
gust test
```

Even if underneath it's doing everything you're already doing.

That abstraction matters tremendously for adoption.

Your underlying architecture can remain sophisticated while your **front door is simple**.

---

# 8. I would also change the mental model presented in the README

Right now the README starts with:

> Test infrastructure for autonomous software...

That's strong.

But then there is a lot of conceptual explanation.

I'd make the first ~30 seconds of the README:

```text
# gust

Behavioral testing infrastructure for AI agents.

Record a real agent run.
Turn it into a reproducible scenario.
Inject failures.
Run the agent repeatedly.
Catch behavioral regressions in CI.

[demo GIF/video]

$ gust test tests/
...
PASS 97/100
FAIL 3/100
```

Then:

```text
Why
How it works
Installation
Existing agent integration
Architecture
Extending Gust
```

Your current documentation is already comprehensive; the issue isn't lack of documentation. It's **time-to-understanding**. ([GitHub][1])

---

# 9. One very important product decision: don't build a UI yet

I would resist the temptation.

Your current positioning is:

```text
Gust ≠ observability platform
Gust ≠ trace database
Gust ≠ hosted dashboard
```

and that is good.

You explicitly position OTel/Phoenix/Langfuse/etc. as production observability infrastructure and Gust as the testing/evaluation sink for controlled sessions. ([GitHub][4])

Keep that boundary.

Your moat should be:

```text
Scenario model
Fixture system
Replay
Mutation
Assertions
Reliability statistics
Regression
CI
Protocols
```

not:

```text
yet another trace viewer
```

---

# 10. There is one architectural evolution I'd prepare for now

**Stateful environments.**

Your current fixture model is already moving in the right direction with:

```text
exact hash
ordered sequence
hybrid matching
```

and this is explicitly part of the specification. ([GitHub][2])

But real agents interact with stateful worlds:

```text
get_cart()
add_item()
get_cart()
remove_item()
checkout()
```

The output of call #4 depends on calls #2 and #3.

So the long-term abstraction shouldn't just be:

```text
tool → response
```

It should become:

```text
Environment
   ├── state
   ├── tools
   ├── resources
   ├── clocks
   ├── network
   └── fault model
```

You're already heading toward this through the scenario/environment model, so I wouldn't redesign anything now. Just make sure the abstraction doesn't accidentally become "mock server with fancy JSON fixtures."

---

# 11. Your biggest strategic advantage

There's an interesting ecosystem position emerging.

Braintrust is explicitly working on **behavior specs** for defining and evaluating agent behavior across entire trajectories. ([GitHub][10])

That's adjacent to Gust, but not identical.

I would actually lean into the distinction:

```text
Behavior specification
        ↓
       Gust
        ↓
Controlled execution
        ↓
Trace
        ↓
Assertions
        ↓
Regression
        ↓
Mutation
```

Gust should be the **execution/testing substrate**, not merely another place where people write evaluation prompts.

That gives you a very clean place in the ecosystem.

---

# 12. What I would do next

Not another giant feature phase.

I'd do a **hardening phase**.

### P0 — correctness

Fix:

```text
tool sequence semantics
tool occurrence semantics
error recovery semantics
latency calculation
strict schema validation
```

These are the things that could make a technically sophisticated user lose trust.

### P1 — adoption

Make:

```bash
gust init
gust test
gust analyze
```

extremely easy.

Give people one realistic end-to-end example with an actual agent, not only a synthetic runner.

The synthetic runner is useful for proving the statistics machinery, but an adopter needs to see:

```text
real LLM
real tool calls
real failure
Gust detects it
```

Your existing architecture already supports this distinction. ([GitHub][1])

### P1 — trust

Make the mutation demo much more prominent.

Something like:

```text
100 mutated trajectories
87 detected
13 escaped

Evaluator mutation score: 87%
```

That tells me something very interesting:

> **Does my test suite actually protect my agent?**

That's a much stronger story than “we have nine evaluators.”

---

# Final architecture assessment

If I saw this project for the first time as a staff/principal engineer, my reaction would be:

**“This person has thought about the problem properly.”**

The strongest evidence is the domain separation:

```text
AgentRun      = what happened
TestScenario  = what should happen
Fixture       = what the world does
Assertion     = what we check
Policy        = what we accept
Replay        = deterministic transformation
Test          = actual agent execution
Mutation      = validate the evaluator
Statistics    = account for nondeterminism
```

That is a coherent system. ([GitHub][2])

The biggest remaining risk isn't that Gust is architecturally wrong.

It's that **the abstraction becomes technically impressive but slightly too complicated for the average agent developer to adopt**, and that **some evaluator semantics aren't yet rigorous enough to earn the “testing infrastructure” label you are claiming.**

Those are fixable.

### My recommendation

**Don't expand the scope yet.**

Take the current implementation and make it **boringly trustworthy and ridiculously easy to try**.

Then I'd be comfortable moving the project from:

> “interesting OSS project”

toward:

> **“this could become a real piece of agent infrastructure.”**

And one final observation: your public repo already says MVP phases 1–9 are complete, Phase 10–11 have landed, and lists Go 1.23+, Python/TypeScript SDKs, OTel HTTP/gRPC, Langfuse ingestion, scenario extraction, mutation, CI and the documentation site. That's a surprisingly broad surface for only 10 public commits, which means the next stage should be consolidation rather than adding breadth. ([GitHub][1])

[1]: https://github.com/ShadyD45/gust "GitHub - ShadyD45/gust: Behavioral testing infrastructure for AI agents. Reproduce runs, inject failures, and catch regressions · GitHub"
[2]: https://github.com/ShadyD45/gust/blob/main/docs/architecture/data-model.md "gust/docs/architecture/data-model.md at main · ShadyD45/gust · GitHub"
[3]: https://github.com/ShadyD45/gust/blob/main/gust_Specification_v0.5.md "gust/gust_Specification_v0.5.md at main · ShadyD45/gust · GitHub"
[4]: https://github.com/ShadyD45/gust/blob/main/docs/usage/otel-ingest.md "gust/docs/usage/otel-ingest.md at main · ShadyD45/gust · GitHub"
[5]: https://github.com/ShadyD45/gust/blob/main/docs/usage/integrate-your-app.md "gust/docs/usage/integrate-your-app.md at main · ShadyD45/gust · GitHub"
[6]: https://github.com/ShadyD45/gust/tree/main/sdk/python "gust/sdk/python at main · ShadyD45/gust · GitHub"
[7]: https://github.com/ShadyD45/gust/tree/main/sdk/typescript "gust/sdk/typescript at main · ShadyD45/gust · GitHub"
[8]: https://github.com/ShadyD45/gust/blob/main/docs/architecture/statistics-and-policy.md "gust/docs/architecture/statistics-and-policy.md at main · ShadyD45/gust · GitHub"
[9]: https://github.com/ShadyD45/gust/blob/main/internal/adapters/evaluators/evaluators.go "gust/internal/adapters/evaluators/evaluators.go at main · ShadyD45/gust · GitHub"
[10]: https://github.com/braintrustdata/agentbehavior?utm_source=chatgpt.com "GitHub - braintrustdata/agentbehavior: Standards for defining and evaluating agent behavior · GitHub"
