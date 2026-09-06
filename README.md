![Gust - Test what happens when your agent meets a world that doesn’t behave.](gust-logo.png)

# gust

**Behavioral testing infrastructure for AI agents.**

Record a real agent run. Turn it into a reproducible scenario. Inject failures. Run the agent repeatedly. Catch behavioral regressions in CI.

gust is **not** a final-answer LLM evaluation library. It measures agent behavior (tool choice, arguments, sequences, safety constraints, latency) and treats correctness as a **statistical pass rate** with confidence intervals.

## Quick start

```bash
# Build the CLI
go build -o gust ./cmd/gust

# Scaffold a project
./gust init

# Run the full MVP demo (recommended)
./demo/run.sh          # Linux/macOS
./demo/run.ps1         # Windows PowerShell
```

```text
$ ./gust mutate testdata/runs/golden_cancel.json
Mutation testing
  Mutants: N | Detected: D | Escaped: E | Skipped: S
  Evaluator mutation score: XX%
  FPR: Y.Y%
```

That score answers: **does my test suite actually protect my agent?**

Or invoke commands directly:

```bash
# Mode 1 — analyze a captured run
./gust analyze testdata/runs/golden_cancel.json --policy testdata/policy.yaml

# Mode 2 — deterministic replay
./gust replay testdata/runs/golden_cancel.json --fixtures testdata/fixtures

# Mode 3 — probabilistic test (synthetic runner; no GPU required)
./gust test testdata/scenarios/cancel_latest_order.yaml --runner synthetic --samples 100

# Mutation trust metric for evaluators
./gust mutate testdata/runs/golden_cancel.json --classes all --n 100

# Baseline vs candidate regression
./gust compare testdata/baseline.json testdata/candidate.json --policy testdata/policy.yaml

# Extract a scenario from a production trace (assertions left empty by design)
./gust scenario from-run testdata/runs/buggy_cancel.json --output out/scenario.yaml
```



### CI exit codes


| Code | Meaning                                               |
| ---- | ----------------------------------------------------- |
| `0`  | Success (or `FLAKY` when `on_flaky: warn` / `ignore`) |
| `1`  | Failure, hard constraint violation, or regression     |
| `2`  | Configuration / runtime error                         |
| `3`  | Flaky failure when `on_flaky: fail`                   |




## Why gust exists

1. **A trace is not the test.** A captured `AgentRun` is evidence of what happened once. A `TestScenario` (task + controlled fixtures + independent assertions) is the test artifact.
2. **Three modes must never be blurred:** Analyze, Replay, and Test. Only Test invokes a live agent/LLM.
3. **Stochastic systems need repeated sampling.** A single PASS/FAIL from one run is a coin flip. gust reports `PASS`, `FAIL`, `FLAKY`, or `INSUFFICIENT_SAMPLES` using Wilson score intervals.



## Execution modes


| Mode        | What it does                                            | Network / LLM  |
| ----------- | ------------------------------------------------------- | -------------- |
| **Analyze** | Evaluate assertions against a captured `AgentRun`       | Offline        |
| **Replay**  | Re-drive recorded tool I/O via fixtures (deterministic) | Offline        |
| **Test**    | Run the real agent against mocked tools, *N* times      | Live agent/LLM |


Mutation testing validates the **deterministic** evaluator suite (Replay-mode). Probabilistic sampling is reserved for Test-mode nondeterminism.

## Use it with your agent

gust reads a JSON document describing what your agent did, so integration is a recorder in your tool-dispatch path — not a rewrite. **[docs/usage/](docs/usage/)** walks through it:


| Guide                                                  | What it covers                                                                                                                                 |
| ------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| [Integrate your app](docs/usage/integrate-your-app.md) | Emit an `AgentRun` from Python, TypeScript, or Go and run your first gate                                                                      |
| [Modes cookbook](docs/usage/modes-cookbook.md)         | Every command, all nine assertion types, fixtures, failure injection, policies                                                                 |
| [OTel ingestion](docs/usage/otel-ingest.md)            | Point an existing OTLP exporter at gust (HTTP/gRPC), or pull a Langfuse trace                                                                  |
| [CI integration](docs/usage/ci-github-actions.md)      | Exit codes; gust on the runner, agent in the job or QA — not production                                                                        |
| [AEE methodology](docs/usage/aee-methodology.md)       | How the self-benchmark is measured                                                                                                             |
| [Benchmarks (site)](docs/benchmarks/)                  | Metrics explained, latest numbers, more proof                                                                                                  |
| [AEE latest results](benchmarks/RESULTS.md)            | Self-report (detection rate, FPR, throughput, H7); see also [benchmark runs](https://github.com/ShadyD45/gust/actions/workflows/benchmark.yml) |




## Extend it

Every pluggable capability is a small Go interface, and the built-ins use the same ones you would. See **[docs/extending/](docs/extending/)**:

- [Custom evaluator (Go)](docs/extending/custom-evaluator-go.md) — assert domain rules the built-ins cannot express
- [Cross-language plugins](docs/extending/wire-plugin-python.md) — reuse Python/TypeScript validation logic over JSON-RPC
- [Custom test runners](docs/extending/custom-test-runner.md) — drive your own agent in Mode 3

Python capture SDK: `[sdk/python/](sdk/python/)`.

## Architecture

See **[docs/architecture/](docs/architecture/)** for system design, data model, statistics, and extension points.

Canonical contracts: [`spec/schemas/`](spec/schemas/) (JSON Schema).  
Documentation site (GitHub Pages): **[https://shadyd45.github.io/gust/](https://shadyd45.github.io/gust/)** — sources in [`docs/`](docs/).  
Demo: [`demo/`](demo/).

## Project layout

```text
cmd/gust/                 CLI entrypoint
pkg/api/                  Public domain types
pkg/jcs/                  RFC 8785 canonical JSON hashing
internal/ports/           Tier-1 Go interfaces
internal/core/            Analyze, Replay, Test, stats, mutate, policy, scenario
internal/adapters/        Evaluators, mutators, fixtures, ingest, testrunner, wire, storage
sdk/python/               Python capture SDK, Mode 3 harness, LangChain adapter
sdk/typescript/           TypeScript capture SDK (stdlib / Node 18+)
spec/schemas/             JSON Schema contracts (Draft 2020-12)
demo/                     End-to-end MVP demo scripts
docs/                     GitHub Pages site (usage, benchmarks, extending, architecture)
benchmarks/               AEE self-report (RESULTS.md updated locally as needed)
```



## Status

MVP Phases 1–9 complete (library, Mode 3, policy, scenario extraction, CLI, demo, CI).

Phase 10–12 have landed (OTel ingest, SDKs, optional LLM judge, AEE self-benchmark). Phase 17 (semantic hardening + adoption front door) has landed. Remaining post-MVP work — stats v2, continuous eval, clustering, multi-agent, real-agent E2E — is tracked in internal engineering plans (maintainers only).

## Self-benchmark (AEE)

gust continuously measures its own evaluation suite (mutation detection / FPR, throughput, reproducibility, Wilson H7). Latest numbers:

**→ [benchmarks/RESULTS.md](benchmarks/RESULTS.md)** · **[benchmark workflow runs](https://github.com/ShadyD45/gust/actions/workflows/benchmark.yml)**

```bash
./gust aee report
```



## Requirements

- Go 1.23+
- Optional: local [Ollama](https://ollama.com) for live Test-mode runs
- Dependencies: Cobra (CLI), yaml.v3 (scenarios/policies). Live OTLP protobuf/gRPC ingest adds the official OTLP proto + gRPC libraries. Core engines remain stdlib-only.



## Documentation site

The `[docs/](docs/)` tree is a [Just the Docs](https://just-the-docs.github.io/just-the-docs/) Jekyll site, published with GitHub Pages (free for public repositories).

```bash
cd docs
bundle install
bundle exec jekyll serve
# http://127.0.0.1:4000/gust/
```

Publish: The workflow is `[.github/workflows/jekyll-gh-pages.yml](.github/workflows/jekyll-gh-pages.yml)` - GitHub’s Jekyll starter, but it builds `docs/` with Bundler so Just the Docs can install. Do not use **Deploy from a branch**; that builder cannot install the theme.

## Contributing

Contributions welcome — see [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow, testing expectations, and the invariants that constrain changes.

## License

Apache-2.0 — see [LICENSE](LICENSE).