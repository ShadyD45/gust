# Contributing to gust

Thanks for considering a contribution. gust is test infrastructure — people will gate production deploys on its verdicts — so the bar for correctness is high and the bar for cleverness is low.

## Quick start

```bash
git clone https://github.com/your-org/gust && cd gust
go test ./...
go build -o gust ./cmd/gust
go build -o gust-aee ./cmd/gust-aee   # internal self-benchmarks (not for end users; not in GitHub Releases)
./demo/run.sh            # Linux/macOS; ./demo/run.ps1 on Windows
```

Tagged releases publish **only** the `gust` binary (see `.goreleaser.yaml` and `.github/workflows/release.yml`).

Self-benchmark checks (maintainers/CI) — golden-suite / validation-catalog regression, not a field proof of Gust on arbitrary agents:

```bash
./gust-aee report
./gust-aee validate
```

Requirements: Go 1.26+ (`go.mod` is authoritative). Ollama is optional and only needed for live Mode 3 runs — the `synthetic` runner covers everything else.

If `go test ./...` and the demo both pass on a fresh clone, your environment is ready.

## Principles that constrain contributions

These are not style preferences. A change that violates one will be asked to change, however good the code is.

1. **A trace is not a test.** Nothing may synthesize assertions from observed agent behavior. Doing so encodes current bugs as the specification. `gust scenario from-run` leaves `assertions: []` deliberately; keep it that way.
2. **The three modes stay separate.** Analyze and Replay never touch the network or invoke an LLM. Only Test mode runs a live agent. Blurring this makes results irreproducible.
3. **Deterministic by default.** Identical inputs produce identical outputs. Repeated Replay runs must be bit-for-bit identical. No wall-clock reads, no unseeded randomness, no map-iteration-order dependence in output.
4. **Dependencies are a cost.** Core engines are stdlib-only. The whole project runs on Cobra and yaml.v3. A new dependency needs a strong justification in the PR description.
5. **Evidence over booleans.** Any new evaluator or check produces structured evidence — expected vs actual, span IDs, diff paths — never just `false`.
6. **Statistics stay honest.** Do not widen a confidence interval, weaken a verdict, or add a fallback that turns `FLAKY` into `PASS` for convenience. `FLAKY` is the correct answer when the data is inconclusive.

## What to work on

- **Roadmap** is tracked in GitHub issues [#1](https://github.com/ShadyD45/gust/issues/1)–[#5](https://github.com/ShadyD45/gust/issues/5) (stats v2, continuous eval, clustering, multi-agent, releases). Public product docs are under [`docs/`](docs/) and publish to GitHub Pages.
- **Good first contributions**: a new deterministic evaluator, a new mutation class, framework adapters or SDK improvements, docs that close a gap you hit while adopting gust.
- **Discuss first**: anything touching the wire protocol, schema versions, verdict semantics, or the statistics engine. Open an issue before writing code — those changes ripple.

## Making a change

### Code

Match the surrounding code. The project is idiomatic Go with small, focused interfaces:

- Ports (interfaces) in [`internal/ports`](internal/ports/); implementations in [`internal/adapters`](internal/adapters/); engines in [`internal/core`](internal/core/); public types in [`pkg/api`](pkg/api/).
- Wrap errors with context: `fmt.Errorf("parse policy %s: %w", path, err)`.
- Accept `context.Context` as the first parameter on anything that can block, and honor cancellation.
- Run `gofmt` before committing. No drive-by refactors: keep a PR to one concern so reviewers can reason about it.

Adding a component? The guides do the walkthrough:

| Component | Guide |
|---|---|
| Evaluator | [docs/extending/custom-evaluator-go.md](docs/extending/custom-evaluator-go.md) |
| Mutator | [docs/extending/custom-evaluator-go.md#adding-a-mutator](docs/extending/custom-evaluator-go.md#adding-a-mutator) |
| Test runner / fixture provider | [docs/extending/custom-test-runner.md](docs/extending/custom-test-runner.md) |
| Cross-language plugin | [docs/extending/wire-plugin-python.md](docs/extending/wire-plugin-python.md) |

### Tests

Every behavior change needs a test. The existing suites are the pattern to follow:

- **Table-driven unit tests** for evaluators and mutators, covering the pass case, the fail case, and the nil/empty-assertion case.
- **Determinism tests** for anything in Analyze or Replay — run twice, assert identical output.
- **Property tests** for statistics, not just examples. Boundary conditions (`k=0`, `k=n`, `n=1`) have burned this codebase before.
- **Golden fixtures** live in [`testdata/`](testdata/). Keep them small and readable; they double as documentation.

If you touch evaluators, run mutation testing to prove your assertions still catch injected bugs:

```bash
./gust mutate testdata/runs/golden_cancel.json --classes all
```

Detection rate must stay ≥ 90% and false positive rate ≤ 5%.

### Schema and protocol changes

Changing `pkg/api` types, `spec/schemas/`, or the wire protocol is a compatibility event:

- **JSON Schema = wire contract; Go `Validate()` = semantic invariants.** Keep assertion-type (and similar) enums in sync; `spec/schemas/schemas_test.go` guards the assertion enum.
- Update the JSON Schema in [`spec/schemas/`](spec/schemas/) alongside the Go type.
- Update the affected docs in the same PR.
- Explain the migration path for existing `run.json` and scenario files in the PR description.
- Content addressing uses RFC 8785 JCS with a deliberate deviation: integers that fit in `int64` are preserved exactly (not coerced through `float64`) so nanosecond timestamps and large IDs hash stably. Do not "fix" that back to pure double semantics without a breaking-change migration.

### Documentation

Docs are part of the product, not an afterthought. When behavior changes, update:

- [`docs/usage/`](docs/usage/) if it changes what users run or see
- [`docs/extending/`](docs/extending/) if it changes an extension contract
- [`docs/architecture/`](docs/architecture/) if it changes the design
- [`README.md`](README.md) if it changes the top-level story

Write examples that actually run. Every command in the docs should be copy-pasteable against `testdata/`.

## Pull requests

1. Branch from `main`: `git checkout -b evaluator-pii-leak`.
2. Keep the PR focused on one concern.
3. Confirm before pushing:
   ```bash
   gofmt -l .          # must print nothing
   go test ./...
   go build -o gust ./cmd/gust && ./demo/run.sh --skip-build
   ```
4. Write a description that answers *why*, not just *what* — what breaks today, what changes, what a reviewer should look at hardest.
5. CI runs tests, build, and the demo smoke test on Linux. It must be green.

Commit messages: a single imperative sentence summarizing the change, with a body when the reasoning is not obvious.

```text
Add PII leak evaluator for outbound tool arguments.

Deterministic regex check over tool span inputs; evidence records
span_id and field name only, never the matched value, since evidence
lands in CI logs.
```

## Security and data hygiene

- **Never commit real production data.** Test fixtures must be synthetic. Trace data from real systems carries user PII by default.
- **No secrets in fixtures, scenarios, or tests** — including in `recorded_response` bodies and `metadata`.
- Evidence maps end up in CI logs and artifacts: record the *location* and *shape* of a violation, never the sensitive value itself.
- For a suspected vulnerability, do not open a public issue. Report it privately to the maintainers.

## Reporting bugs

A useful report includes the gust command you ran, the minimal `run.json` or scenario that reproduces it, what you expected, what happened, and the `--json` output. Redact anything sensitive before pasting — and if redacting is hard, that is itself a bug worth telling us about.

## License

Contributions are licensed under [Apache-2.0](LICENSE), the same as the project.
