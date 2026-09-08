#!/usr/bin/env bash
# Thin wrapper: live-eval adoption path is demo/live-agent integration scenarios.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
exec "$ROOT/demo/live-agent/run.sh" --only integration,integration-unsafe "$@"
