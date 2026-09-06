---
title: CI integration
nav_order: 4
parent: Usage
---
# Running gust in CI

gust is built for CI: one static binary, offline by default, and exit codes that map cleanly onto pipeline decisions.

## Exit codes

| Code | Meaning | Typical CI response |
|---|---|---|
| `0` | Gate passed (including `FLAKY` under `on_flaky: warn` / `ignore`) | Green |
| `1` | `FAIL`, hard constraint violation, or regression | Red — block the merge |
| `2` | Configuration or runtime error (bad path, unparseable input) | Red — fix the pipeline, not the agent |
| `3` | `FLAKY` under `on_flaky: fail` | Red, but distinguishable from a proven failure |

Code `3` exists so you can treat "we cannot prove this is reliable" differently from "this is broken" — for example, blocking the merge but routing to a different alert channel.

## The minimal PR gate

Analyze mode needs no LLM, no network, and no secrets, so it belongs on every pull request:

```yaml
name: agent-behavior
on: [pull_request]

jobs:
  gate:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"

      - name: Build gust
        run: go build -o gust ./cmd/gust

      - name: Capture agent traces
        run: python scripts/record_traces.py --out runs/    # your recorder

      - name: Gate behavior
        run: |
          for run in runs/*.json; do
            ./gust analyze "$run" --assertions tests/assertions.json --policy policy.yaml
          done
```

If you consume gust from another repository, replace the build step with a download of a released binary and keep everything else identical.

## Reliability gate with a step summary

`gust test` writes a markdown table to `$GITHUB_STEP_SUMMARY` automatically when that variable is set (non-`--json` output), so the verdict shows up on the run page instead of buried in logs:

```yaml
      - name: Reliability gate
        run: ./gust test tests/cancel_order.yaml --runner synthetic --samples 100 --policy policy.yaml
```

| Scenario | Samples | Pass Rate | 95% CI | Verdict |
|---|---|---|---|---|
| `cancel_latest_order` | 100 | 100.0% | `[96.3%, 100.0%]` | ✅ **PASS** |

To keep the machine-readable evidence as well, run once with `--json` and upload it:

```yaml
      - name: Reliability gate
        run: ./gust test tests/cancel_order.yaml --samples 100 --policy policy.yaml --json > reliability.json

      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: gust-reliability
          path: reliability.json
```

## Regression gate against main

Store the main-branch numbers as a baseline artifact, then compare each PR against it:

```yaml
      - name: Download baseline
        uses: actions/download-artifact@v4
        with:
          name: gust-baseline
          path: baseline/
        continue-on-error: true

      - name: Compare
        if: hashFiles('baseline/baseline.json') != ''
        run: ./gust compare baseline/baseline.json candidate.json --policy policy.yaml
```

`baseline.json` and `candidate.json` are the small summary shape described in [Regression comparison]({% link usage/modes-cookbook.md %}#regression-comparison): `passes`, `samples`, `pass_rate`, `latency_ns`. Generate them from your `--json` reliability output.

## Handling flaky verdicts explicitly

When you set `on_flaky: fail`, branch on the exit code so an inconclusive result reads differently from a proven failure:

```yaml
      - name: Reliability gate
        run: |
          set +e
          ./gust test tests/cancel_order.yaml --samples 100 --policy strict-policy.yaml
          code=$?
          set -e
          case $code in
            0) echo "Gate passed" ;;
            3) echo "::warning::Reliability inconclusive (FLAKY) — raise samples or fix the agent"; exit 1 ;;
            *) echo "::error::Agent behavior gate failed"; exit $code ;;
          esac
```

Simpler alternative, if you do not need the distinction in-pipeline: leave `on_flaky: warn` while adopting, then flip to `fail` once your suite is stable.

## Live agent tests

Analyze, Replay, Compare, and Mutate never touch the network — run them on every commit. Mode 3 against a real LLM costs time and money, so gate it:

```yaml
  live:
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    steps:
      # ... build ...
      - name: Nightly reliability
        run: ./gust test tests/cancel_order.yaml --runner ollama --endpoint "$OLLAMA_URL" --samples 200
        env:
          OLLAMA_URL: ${{ secrets.OLLAMA_URL }}
```

A local model via Ollama keeps this near zero cost. Point tool calls at the fixture proxy (`AGENTEVAL_FIXTURE_ENDPOINT`) so the agent is live but its dependencies are not.

## Keeping CI cheap

This repository's own workflow, [`.github/workflows/ci.yml`](https://github.com/ShadyD45/gust/blob/main/.github/workflows/ci.yml), is a working example of the cost discipline worth copying:

- `paths-ignore` for docs and images so documentation changes do not burn runner minutes.
- `concurrency` with `cancel-in-progress` to kill superseded runs.
- Linux-only (Windows runners bill at 2x, macOS at 10x).
- Build the binary once and pass it to downstream steps with `--bin` rather than rebuilding.

## Other CI systems

Nothing here is GitHub-specific except the step summary. The pattern is always: build or download the binary, run a gust command, let the exit code decide.

```yaml
# GitLab CI
agent-behavior:
  image: golang:1.26
  script:
    - go build -o gust ./cmd/gust
    - ./gust analyze runs/latest.json --policy policy.yaml
```

```groovy
// Jenkins
sh 'go build -o gust ./cmd/gust'
sh './gust analyze runs/latest.json --policy policy.yaml'
```

