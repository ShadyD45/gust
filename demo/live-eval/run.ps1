# Thin wrapper: live-eval adoption path is demo/live-agent integration scenarios.
param(
  [Parameter(ValueFromRemainingArguments = $true)]
  [string[]]$Rest
)
$ErrorActionPreference = "Stop"
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
& (Join-Path $Root "demo\live-agent\run.ps1") -Only "integration,integration-unsafe" @Rest
exit $LASTEXITCODE
