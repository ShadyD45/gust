![Gust - Test what happens when your agent meets a world that doesn’t behave.](gust-logo.png)

**Test infrastructure for autonomous software** - the role that JUnit, a mocking framework, and a CI regression gate play for ordinary backends, adapted for LLM-driven agents that do not give the same answer twice.

gust is **not** a final-answer LLM evaluation library. It measures agent behavior (tool choice, arguments, sequences, safety constraints, latency) and treats correctness as a **statistical pass rate** with confidence intervals.

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

## Quick start

```bash
# Build the CLI
go build -o gust ./cmd/gust

# Run the full MVP killer demo (recommended)
./demo/run.sh          # Linux/macOS
./demo/run.ps1         # Windows PowerShell
```

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


## Use it with your agent

gust reads a JSON document describing what your agent did, so integration is a recorder in your tool-dispatch path — not a rewrite. **[docs/usage/](docs/usage/)** walks through it:


| Guide                                                  | What it covers                                                                 |
| ------------------------------------------------------ | ------------------------------------------------------------------------------ |
| [Integrate your app](docs/usage/integrate-your-app.md) | Emit an `AgentRun` from Python, TypeScript, or Go and run your first gate      |
| [Modes cookbook](docs/usage/modes-cookbook.md)         | Every command, all nine assertion types, fixtures, failure injection, policies |
| [OTel ingestion](docs/usage/otel-ingest.md)            | Convert OpenTelemetry / OpenInference traces instead of writing a recorder     |
| [CI integration](docs/usage/ci-github-actions.md)      | Exit codes, step summaries, regression baselines                               |


## Extend it

Every pluggable capability is a small Go interface, and the built-ins use the same ones you would. See **[docs/extending/](docs/extending/)**:

- [Custom evaluator (Go)](docs/extending/custom-evaluator-go.md) — assert domain rules the built-ins cannot express
- [Cross-language plugins](docs/extending/wire-plugin-python.md) — reuse Python/TypeScript validation logic over JSON-RPC
- [Custom test runners](docs/extending/custom-test-runner.md) — drive your own agent in Mode 3

Python capture SDK: [`sdk/python/`](sdk/python/).

## Architecture

See **[docs/architecture/](docs/architecture/)** for system design, data model, statistics, and extension points.

Canonical requirements: [`gust_Specification_v0.5.md`](gust_Specification_v0.5.md).  
Documentation site (GitHub Pages): **https://shadyd45.github.io/gust/** — sources in [`docs/`](docs/).  
Killer demo: [`demo/`](demo/).

## Project layout

```text
cmd/gust/                 CLI entrypoint
pkg/api/                  Public domain types
pkg/jcs/                  RFC 8785 canonical JSON hashing
internal/ports/           Tier-1 Go interfaces
internal/core/            Analyze, Replay, Test, stats, mutate, policy, scenario
internal/adapters/        Evaluators, mutators, fixtures, ingest, testrunner, wire, storage
sdk/python/               Python capture SDK + wire plugin example
spec/schemas/             JSON Schema contracts (Draft 2020-12)
demo/                     End-to-end MVP demo scripts
docs/                     GitHub Pages site (usage, extending, architecture)
```

## Status

MVP Phases 1–9 complete (library, Mode 3, policy, scenario extraction, CLI, demo, CI).

Phase 10 (OTel/OpenInference file ingestion) and the Phase 11 Python capture SDK have landed. Remaining post-MVP work — TypeScript SDK, framework adapters, stats v2, continuous eval, clustering, multi-agent/AEE — is tracked in internal engineering plans (maintainers only).

## Requirements

- Go 1.23+
- Optional: local [Ollama](https://ollama.com) for live Test-mode runs
- Dependencies kept minimal: Cobra (CLI), yaml.v3 (scenarios/policies). Core engines are stdlib-only.

## Documentation site

The [`docs/`](docs/) tree is a [Just the Docs](https://just-the-docs.github.io/just-the-docs/) Jekyll site, published with GitHub Pages (free for public repositories).

```bash
cd docs
bundle install
bundle exec jekyll serve
# http://127.0.0.1:4000/gust/
```

After the first `pages` workflow run, set **Settings → Pages → Source** to **GitHub Actions** if prompted.

## Contributing

Contributions welcome — see [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow, testing expectations, and the invariants that constrain changes.

## License

Apache-2.0 — see [LICENSE](LICENSE).