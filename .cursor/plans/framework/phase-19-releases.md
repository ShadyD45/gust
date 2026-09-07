# Phase 19: Public Releases & Binary Split (`gust` vs `gust-aee`)

**Status:** Planned (partially started — binary split landed ahead of packaging)  
**Track:** Framework / Distribution  
**Depends on:** Phase 9 (CLI), Phase 16 (AEE self-benchmark), Validation Suite

---

## Intent

Ship **gust** as the only end-user installable tool (GitHub Releases, Homebrew, Scoop, etc.). Keep **gust-aee** as a maintainer/CI binary that is built from this repo but **not** published through package managers.

```text
End users                         Maintainers / CI
─────────                         ────────────────
gust                              gust-aee
  analyze / replay / test          report   (AEE metrics)
  mutate / compare / scenario        validate (adversarial suite)
  ingest / judge / init
```

Users should never need `gust-aee` to test their agents. Self-trust proof stays internal.

---

## Already done (prerequisite)

- [x] Separate entrypoint [`cmd/gust-aee`](../../cmd/gust-aee)
- [x] AEE + Validation Suite removed from end-user `gust` command tree
- [x] Commands: `gust-aee report`, `gust-aee validate`
- [x] Benchmark workflow builds/runs `gust-aee` (not `gust aee …`)

---

## Scope

### 19.1 Release artifacts (end-user `gust` only)

| Deliverable | Notes |
|-------------|--------|
| GitHub Release on tagged `vX.Y.Z` | Source of truth for downloadable binaries |
| Cross-compile matrix | linux/darwin/windows × amd64/arm64 (as relevant) |
| Checksums | `SHA256SUMS` (or cosign later) |
| Changelog | Keep a human `CHANGELOG.md` or release notes from commits |
| **Do not** attach `gust-aee` to public release assets | Build it in CI for gates only |

Suggested layout for release assets:

```text
gust_v1.2.3_linux_amd64.tar.gz
gust_v1.2.3_darwin_arm64.tar.gz
gust_v1.2.3_windows_amd64.zip
SHA256SUMS
```

### 19.2 Package managers (end-user `gust` only)

| Channel | Priority | Notes |
|---------|----------|--------|
| Homebrew (tap or core) | P0 | `brew install …` |
| Scoop (Windows) | P1 | Optional once Windows users show up |
| `go install github.com/ShadyD45/gust/cmd/gust@latest` | P0 | Document; always available for Go users |
| npm/pip wrappers | Out of scope | SDKs are separate; CLI stays a Go binary |

Explicit non-goal: packaging or advertising `gust-aee` on Homebrew/Scoop/GitHub Releases.

### 19.3 CI / release automation

- [ ] `release.yml` (or goreleaser) on `v*` tags
- [ ] Smoke-test the released `gust` binary (`gust version` / `gust --help`)
- [ ] Keep `benchmark.yml` building **both** binaries; publish artifacts for AEE JSON only (not the `gust-aee` binary as a product download)
- [ ] Document versioning: semver; breaking schema/wire changes bump major

### 19.4 Docs & DX

- [ ] README install section: download / brew / `go install` for **gust only**
- [ ] CONTRIBUTING: how maintainers build `gust-aee` locally
- [ ] Clarify on docs site: “Self-benchmarks use `gust-aee` in CI; you do not install it”
- [ ] Optional: `gust version` prints build version/commit for support

### 19.5 Security / trust for releases

- [ ] Pin Go toolchain in release workflow
- [ ] Reproducible or at least documented build flags (`-trimpath`, ldflags version)
- [ ] Later: signed releases (cosign / GitHub attestations) — not required for first tag

---

## Exit criteria

1. Tagged release publishes **only** `gust` multi-arch binaries + checksums.
2. At least one package manager path works for `gust` (Homebrew **or** documented `go install`).
3. `gust-aee` remains buildable from source and used in CI; absent from public install docs and release assets.
4. Docs never instruct end users to run AEE/validation via the product CLI.

---

## Suggested sequencing

```text
Binary split (done) ──► goreleaser / tag v0.x ──► Homebrew tap
                              │
                              └──► version ldflags + CHANGELOG
```

---

## Out of scope for this phase

- Hosted “Gust Cloud” / SaaS distribution
- Publishing Python/TS SDKs to PyPI/npm as part of the same release train (separate cadence OK)
- Full OS sandboxing for Tier-2 plugins (trust-boundary phase)
