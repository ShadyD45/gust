param(
  [switch]$SkipBuild,
  [string]$Bin = ""
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
if (-not $Root) { $Root = (Resolve-Path "$PSScriptRoot\..").Path }
Set-Location $Root

$Demo = Join-Path $Root "demo"
$DefaultBin = Join-Path $Root "gust.exe"

# Env fallbacks when flags omitted
if (-not $Bin -and $env:GUST_BIN) {
  $Bin = $env:GUST_BIN
  $SkipBuild = $true
}
if ($env:SKIP_BUILD -eq "1" -or $env:SKIP_BUILD -eq "true") {
  $SkipBuild = $true
}

if ($Bin) {
  if (-not (Test-Path $Bin)) {
    Write-Error "binary not found: $Bin"
    exit 2
  }
  $Gust = $Bin
} elseif ($SkipBuild) {
  if (-not (Test-Path $DefaultBin)) {
    Write-Error "missing $DefaultBin; build first or omit -SkipBuild"
    exit 2
  }
  $Gust = $DefaultBin
} else {
  Write-Host "==> Building gust"
  go build -o gust.exe ./cmd/gust
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
  $Gust = $DefaultBin
}

Write-Host "==> Using binary: $Gust"

Write-Host ""
Write-Host "==> 1/5 Mutation testing"
& $Gust mutate (Join-Path $Demo "golden_cancel.json")
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host ""
Write-Host "==> 2/5 Probabilistic test (synthetic, N=100)"
& $Gust test (Join-Path $Demo "cancel_latest_order.yaml") `
  --runner synthetic `
  --samples 100 `
  --pass-probability 1.0 `
  --policy (Join-Path $Demo "policy.yaml")
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host ""
Write-Host "==> 3/5 Regression compare (expect failure / exit 1)"
& $Gust compare (Join-Path $Demo "baseline.json") (Join-Path $Demo "candidate.json") --policy (Join-Path $Demo "policy.yaml")
if ($LASTEXITCODE -ne 1) {
  Write-Error "expected compare exit code 1 (regression), got $LASTEXITCODE"
  exit 1
}
Write-Host "compare correctly reported regression (exit 1)"

Write-Host ""
Write-Host "==> 4/5 Analyze + Replay"
& $Gust analyze (Join-Path $Demo "golden_cancel.json")
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
& $Gust replay (Join-Path $Demo "golden_cancel.json") --fixtures (Join-Path $Demo "fixtures")
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host ""
Write-Host "==> 5/5 Scenario extraction (H8: empty assertions)"
$OutDir = Join-Path $Demo "out"
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$Extracted = Join-Path $OutDir "extracted.yaml"
& $Gust scenario from-run (Join-Path $Demo "buggy_cancel.json") --output $Extracted
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
$content = Get-Content $Extracted -Raw
if ($content -notmatch "Trace is not the test") {
  Write-Error "H8 check failed: missing extraction warning"
  exit 1
}
Write-Host "H8 ok: extraction warning present"

Write-Host ""
Write-Host "Demo complete - MVP workflows verified."
