$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
if (-not $Root) { $Root = (Resolve-Path "$PSScriptRoot\..").Path }
Set-Location $Root

Write-Host "==> Building gust"
go build -o gust.exe ./cmd/gust

$Demo = Join-Path $Root "demo"
$Gust = Join-Path $Root "gust.exe"

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
Write-Host "Demo complete - MVP killer workflows verified."
