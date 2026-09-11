# Changelog

All notable changes to the **gust** CLI and libraries are documented here.
Maintainer-only `gust-aee` changes may appear under Benchmarks but are not end-user release notes.

## Unreleased

### Added

- Stats v2: seedable bootstrap CIs, Cohen's d, `gust recommend-samples`, richer `gust compare`
- Continuous eval: `gust scenario propose` with default PII redaction and audited opt-out
- Failure clustering: `gust cluster runs` with deterministic fingerprints
- Multi-agent assertions: `agent_handoff`, `role_adherence`, `coordination_order`
- AEE hardening extras (per-mutator breakdown, Analyze latency, replay identity)
- Public release automation for `gust` via GoReleaser (GitHub Releases on `v*` tags; `gust-aee` excluded)

### Fixed / hardened

- Documentation truth audit (Go 1.26+, assertion catalogue wording, independence caveats)
- Mode 3 stateful counter isolation stress test
- SampleResult `attempts` / `retry_count`
- Privacy and invariants architecture docs

## 0.5.x

Initial public MVP line aligned with `schema_version: "0.5"`.
