# Phase 15: Failure Mining and Clustering

## Objectives

1. Cluster similar production failures to avoid duplicate scenario proposals.
2. Deduplicate by structural trace features first; optional embeddings later.
3. Feed Phase 14 proposers with “one representative failure per cluster.”

## Scope

| In | Out |
|----|-----|
| Feature extraction from `AgentRun` (tools, args shapes, errors) | Full vector DB product |
| Offline clustering job + report | Auto-merge into golden datasets |
| Cluster IDs in provenance metadata | Replacing human review |

## Design notes

- Start with deterministic fingerprints (tool sequence + error codes + arg schema hashes).
- Optional embedding/DBSCAN path behind a feature flag once SDKs exist.

## Verification

- Near-duplicate buggy runs collapse to one cluster.
- Distinct failure modes remain separated (confusion matrix on labeled set).

## Exit criteria

`gust mine failures --input runs/ --out clusters.json` produces stable, reviewable clusters.
