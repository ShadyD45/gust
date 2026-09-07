#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

LIVE="$ROOT/demo/live-agent"
DEFAULT_BIN="$ROOT/gust"
POLICY="$LIVE/policy.yaml"
OUT_DIR="$ROOT/demo/out"

skip_build=0
bin_arg=""
mode="scripted"
model="${GUST_OLLAMA_MODEL:-llama3.2:3b}"
host="${OLLAMA_HOST:-http://127.0.0.1:11434}"
samples=20
concurrency=""
timeout=""
only="healthy,recovery,buggy,unsafe"

usage() {
  cat <<'EOF'
Usage: ./demo/live-agent/run.sh [options]

Run the live-agent demo against Mode 3 fixtures. Default is the scripted
path (no GPU). Use --ollama to drive a local model through the same
scenarios.

  --scripted          Scripted agent (default). Unsets GUST_LIVE_AGENT.
  --ollama            Real Ollama model (sets GUST_LIVE_AGENT=1)
  --model <name>      Ollama model (default: llama3.2:3b)
  --host <url>        Ollama host (default: http://127.0.0.1:11434)
  --samples <n>       Wilson sample count (default: 20)
  --concurrency <n>   Workers (default: 4 scripted, 1 ollama)
  --timeout <sec>     Per-sample timeout (default: 60 scripted, 180 ollama)
  --only <list>       Comma list: healthy,recovery,buggy,unsafe
  --skip-build        Reuse ./gust (or GUST_BIN)
  --bin <path>        Use this binary (implies skip build)
  -h, --help

Expects: healthy + recovery PASS; buggy + unsafe FAIL (hard / reliability).
JSON reports land in demo/out/.

Env: GUST_BIN, SKIP_BUILD=1, GUST_OLLAMA_MODEL, OLLAMA_HOST
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --scripted)
      mode="scripted"
      shift
      ;;
    --ollama)
      mode="ollama"
      shift
      ;;
    --model)
      model="${2:-}"
      [[ -n "$model" ]] || { echo "--model requires a name" >&2; exit 2; }
      shift 2
      ;;
    --host)
      host="${2:-}"
      [[ -n "$host" ]] || { echo "--host requires a URL" >&2; exit 2; }
      shift 2
      ;;
    --samples)
      samples="${2:-}"
      [[ -n "$samples" ]] || { echo "--samples requires n" >&2; exit 2; }
      shift 2
      ;;
    --concurrency)
      concurrency="${2:-}"
      [[ -n "$concurrency" ]] || { echo "--concurrency requires n" >&2; exit 2; }
      shift 2
      ;;
    --timeout)
      timeout="${2:-}"
      [[ -n "$timeout" ]] || { echo "--timeout requires seconds" >&2; exit 2; }
      shift 2
      ;;
    --only)
      only="${2:-}"
      [[ -n "$only" ]] || { echo "--only requires a list" >&2; exit 2; }
      shift 2
      ;;
    --skip-build)
      skip_build=1
      shift
      ;;
    --bin)
      bin_arg="${2:-}"
      [[ -n "$bin_arg" ]] || { echo "--bin requires a path" >&2; exit 2; }
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

if [[ -z "$bin_arg" && -n "${GUST_BIN:-}" ]]; then
  bin_arg="$GUST_BIN"
  skip_build=1
fi
if [[ "${SKIP_BUILD:-}" == "1" || "${SKIP_BUILD:-}" == "true" ]]; then
  skip_build=1
fi

if [[ -z "$concurrency" ]]; then
  if [[ "$mode" == "ollama" ]]; then
    concurrency=1
  else
    concurrency=4
  fi
fi
if [[ -z "$timeout" ]]; then
  if [[ "$mode" == "ollama" ]]; then
    timeout=180
  else
    timeout=60
  fi
fi

resolve_gust() {
  if [[ -n "$bin_arg" ]]; then
    if [[ ! -x "$bin_arg" && ! -f "$bin_arg" ]]; then
      echo "binary not found: $bin_arg" >&2
      exit 2
    fi
    echo "$bin_arg"
    return
  fi
  if [[ "$skip_build" -eq 1 ]]; then
    if [[ -x "$DEFAULT_BIN" ]]; then
      echo "$DEFAULT_BIN"
      return
    fi
    if [[ -f "${DEFAULT_BIN}.exe" ]]; then
      echo "${DEFAULT_BIN}.exe"
      return
    fi
    echo "missing $DEFAULT_BIN; build first or omit --skip-build" >&2
    exit 2
  fi
  echo "==> Building gust" >&2
  if [[ "${OS:-}" == "Windows_NT" ]]; then
    go build -o gust.exe ./cmd/gust
    echo "${DEFAULT_BIN}.exe"
  else
    go build -o gust ./cmd/gust
    echo "$DEFAULT_BIN"
  fi
}

want() {
  local name="$1"
  IFS=',' read -r -a parts <<<"$only"
  local p
  for p in "${parts[@]}"; do
    p="$(echo "$p" | tr -d '[:space:]')"
    if [[ "$p" == "$name" ]]; then
      return 0
    fi
  done
  return 1
}

GUST="$(resolve_gust)"
mkdir -p "$OUT_DIR"

if [[ "$mode" == "ollama" ]]; then
  export GUST_LIVE_AGENT=1
  export GUST_OLLAMA_MODEL="$model"
  export OLLAMA_HOST="$host"
  echo "==> Live Ollama ($model @ $host)"
else
  unset GUST_LIVE_AGENT || true
  echo "==> Scripted agent (no model)"
fi
echo "==> Using binary: $GUST"
echo "==> samples=$samples concurrency=$concurrency timeout=${timeout}s policy=$POLICY"

run_pass() {
  local name="$1"
  local json="$OUT_DIR/live-${name}.json"
  echo ""
  echo "==> $name (expect PASS)"
  "$GUST" test "$LIVE/$name" \
    --policy "$POLICY" \
    --samples "$samples" \
    --concurrency "$concurrency" \
    --timeout "$timeout" \
    --json | tee "$json"
}

run_fail() {
  local name="$1"
  local json="$OUT_DIR/live-${name}.json"
  echo ""
  echo "==> $name (expect FAIL)"
  set +e
  "$GUST" test "$LIVE/$name" \
    --policy "$POLICY" \
    --samples "$samples" \
    --concurrency "$concurrency" \
    --timeout "$timeout" \
    --json | tee "$json"
  local rc=$?
  set -e
  if [[ "$rc" -eq 0 ]]; then
    echo "expected $name to fail" >&2
    exit 1
  fi
  echo "$name failed as expected (exit $rc)"
}

if want healthy; then run_pass healthy; fi
if want recovery; then run_pass recovery; fi
if want buggy; then run_fail buggy; fi
if want unsafe; then run_fail unsafe; fi

echo ""
echo "Live-agent demo complete ($mode, N=$samples). Reports in $OUT_DIR/"
