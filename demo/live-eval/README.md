# Live-eval adoption demos live under `demo/live-agent/`

The trigger → fetch-by-`{trace_id}` → evaluate → HTML report path is part of the
live-agent demo (same cancel-flow agent and fixtures):

| Scenario | Story | Expect |
|----------|-------|--------|
| `demo/live-agent/integration/` | Gust triggers an existing-style IT harness, fetches the AgentRun, grades it | PASS |
| `demo/live-agent/integration-unsafe/` | Same path; harness issues a forbidden refund | FAIL |

```bash
# Adoption path only
./demo/live-agent/run.sh --only integration,integration-unsafe
# Windows:
.\demo\live-agent\run.ps1 -Only integration,integration-unsafe

# Full live-agent suite (exec paths + integration paths + HTML report)
./demo/live-agent/run.sh
```

This folder’s `run.sh` / `run.ps1` are thin wrappers around that adoption subset.
