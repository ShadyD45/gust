param(
  [switch]$SkipBuild,
  [string]$Bin = "",
  [switch]$Scripted,
  [switch]$Ollama,
  [string]$Model = "",
  [string]$OllamaHost = "",
  [int]$Samples = 20,
  [int]$Concurrency = 0,
  [int]$Timeout = 0,
  [string]$Only = "healthy,recovery,buggy,unsafe"
)

$ErrorActionPreference = "Stop"
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
Set-Location $Root

$Live = Join-Path $Root "demo\live-agent"
$DefaultBin = Join-Path $Root "gust.exe"
$Policy = Join-Path $Live "policy.yaml"
$OutDir = Join-Path $Root "demo\out"

if ($Ollama -and $Scripted) {
  Write-Error "use either -Ollama or -Scripted, not both"
  exit 2
}
$Mode = "scripted"
if ($Ollama) { $Mode = "ollama" }

if (-not $Model -and $env:GUST_OLLAMA_MODEL) { $Model = $env:GUST_OLLAMA_MODEL }
if (-not $Model) { $Model = "llama3.2:3b" }
if (-not $OllamaHost -and $env:OLLAMA_HOST) { $OllamaHost = $env:OLLAMA_HOST }
if (-not $OllamaHost) { $OllamaHost = "http://127.0.0.1:11434" }

if (-not $Bin -and $env:GUST_BIN) {
  $Bin = $env:GUST_BIN
  $SkipBuild = $true
}
if ($env:SKIP_BUILD -eq "1" -or $env:SKIP_BUILD -eq "true") {
  $SkipBuild = $true
}

if ($Concurrency -le 0) {
  $Concurrency = if ($Mode -eq "ollama") { 1 } else { 4 }
}
if ($Timeout -le 0) {
  $Timeout = if ($Mode -eq "ollama") { 180 } else { 60 }
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

function Test-Wanted([string]$Name) {
  $parts = $Only.Split(",") | ForEach-Object { $_.Trim() }
  return $parts -contains $Name
}

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

if ($Mode -eq "ollama") {
  $env:GUST_LIVE_AGENT = "1"
  $env:GUST_OLLAMA_MODEL = $Model
  $env:OLLAMA_HOST = $OllamaHost
  Write-Host "==> Live Ollama ($Model @ $OllamaHost)"
} else {
  Remove-Item Env:GUST_LIVE_AGENT -ErrorAction SilentlyContinue
  Write-Host "==> Scripted agent (no model)"
}
Write-Host "==> Using binary: $Gust"
Write-Host "==> samples=$Samples concurrency=$Concurrency timeout=${Timeout}s policy=$Policy"

function Invoke-Pass([string]$Name) {
  $json = Join-Path $OutDir "live-$Name.json"
  Write-Host ""
  Write-Host "==> $Name (expect PASS)"
  & $Gust test (Join-Path $Live $Name) `
    --policy $Policy `
    --samples $Samples `
    --concurrency $Concurrency `
    --timeout $Timeout `
    --json | Tee-Object -FilePath $json
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

function Invoke-Fail([string]$Name) {
  $json = Join-Path $OutDir "live-$Name.json"
  Write-Host ""
  Write-Host "==> $Name (expect FAIL)"
  & $Gust test (Join-Path $Live $Name) `
    --policy $Policy `
    --samples $Samples `
    --concurrency $Concurrency `
    --timeout $Timeout `
    --json | Tee-Object -FilePath $json
  if ($LASTEXITCODE -eq 0) {
    Write-Error "expected $Name to fail"
    exit 1
  }
  Write-Host "$Name failed as expected (exit $LASTEXITCODE)"
}

if (Test-Wanted "healthy") { Invoke-Pass "healthy" }
if (Test-Wanted "recovery") { Invoke-Pass "recovery" }
if (Test-Wanted "buggy") { Invoke-Fail "buggy" }
if (Test-Wanted "unsafe") { Invoke-Fail "unsafe" }

Write-Host ""
Write-Host "Live-agent demo complete ($Mode, N=$Samples). Reports in $OutDir/"
exit 0
