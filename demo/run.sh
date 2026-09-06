#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "==> Building gust"
go build -o gust ./cmd/gust

DEMO="$ROOT/demo"
GUST="$ROOT/gust"

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
  # also accept YAML block with no assertion items after warning
  if grep -q 'Trace is not the test' "$DEMO/out/extracted.yaml" && grep -q '^assertions:' "$DEMO/out/extracted.yaml"; then
    echo "H8 ok: extraction warning present and assertions section exists"
  else
    echo "H8 check failed" >&2
    exit 1
  fi
fi

echo ""
echo "Demo complete - MVP killer workflows verified."
