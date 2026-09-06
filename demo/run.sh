#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

DEMO="$ROOT/demo"
DEFAULT_BIN="$ROOT/gust"
skip_build=0
bin_arg=""

usage() {
  cat <<'EOF'
Usage: ./demo/run.sh [--skip-build] [--bin <path>]

  (default)       Build ./gust then run the demo
  --skip-build    Reuse ./gust (or GUST_BIN); fail if missing
  --bin <path>    Use this binary (implies skip build)

Env fallbacks: GUST_BIN, SKIP_BUILD=1
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-build)
      skip_build=1
      shift
      ;;
    --bin)
      bin_arg="${2:-}"
      if [[ -z "$bin_arg" ]]; then
        echo "--bin requires a path" >&2
        exit 2
      fi
      skip_build=1
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

# Env fallbacks when flags omitted
if [[ -z "$bin_arg" && -n "${GUST_BIN:-}" ]]; then
  bin_arg="$GUST_BIN"
  skip_build=1
fi
if [[ "${SKIP_BUILD:-}" == "1" || "${SKIP_BUILD:-}" == "true" ]]; then
  skip_build=1
fi

resolve_gust() {
  if [[ -n "$bin_arg" ]]; then
    if [[ ! -x "$bin_arg" ]]; then
      echo "binary not found or not executable: $bin_arg" >&2
      exit 2
    fi
    echo "$bin_arg"
    return
  fi
  if [[ "$skip_build" -eq 1 ]]; then
    if [[ ! -x "$DEFAULT_BIN" ]]; then
      echo "missing $DEFAULT_BIN; build first or omit --skip-build" >&2
      exit 2
    fi
    echo "$DEFAULT_BIN"
    return
  fi
  echo "==> Building gust" >&2
  go build -o gust ./cmd/gust
  echo "$DEFAULT_BIN"
}

GUST="$(resolve_gust)"
echo "==> Using binary: $GUST"

echo ""
echo "==> 1/5 Mutation testing"
"$GUST" mutate "$DEMO/golden_cancel.json"

echo ""
echo "==> 2/5 Probabilistic test (synthetic, N=100)"
"$GUST" test "$DEMO/cancel_latest_order.yaml" \
  --runner synthetic \
  --samples 100 \
  --pass-probability 1.0 \
  --policy "$DEMO/policy.yaml"

echo ""
echo "==> 3/5 Regression compare (expect failure / exit 1)"
set +e
"$GUST" compare "$DEMO/baseline.json" "$DEMO/candidate.json" --policy "$DEMO/policy.yaml"
cmp_rc=$?
set -e
if [[ "$cmp_rc" -ne 1 ]]; then
  echo "expected compare exit code 1 (regression), got $cmp_rc" >&2
  exit 1
fi
echo "compare correctly reported regression (exit 1)"

echo ""
echo "==> 4/5 Analyze + Replay"
"$GUST" analyze "$DEMO/golden_cancel.json"
"$GUST" replay "$DEMO/golden_cancel.json" --fixtures "$DEMO/fixtures"

echo ""
echo "==> 5/5 Scenario extraction (H8: empty assertions)"
mkdir -p "$DEMO/out"
"$GUST" scenario from-run "$DEMO/buggy_cancel.json" --output "$DEMO/out/extracted.yaml"
if grep -q 'assertions: \[\]' "$DEMO/out/extracted.yaml" || grep -A2 '^assertions:' "$DEMO/out/extracted.yaml" | grep -q '\[\]'; then
  echo "H8 ok: assertions left empty"
else
  if grep -q 'Trace is not the test' "$DEMO/out/extracted.yaml" && grep -q '^assertions:' "$DEMO/out/extracted.yaml"; then
    echo "H8 ok: extraction warning present and assertions section exists"
  else
    echo "H8 check failed" >&2
    exit 1
  fi
fi

echo ""
echo "Demo complete - MVP workflows verified."
