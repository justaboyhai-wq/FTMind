param(
  [Parameter(Mandatory=$true)][string]$DataDir,
  [Parameter(Mandatory=$true)][string]$OutputDir
)

$ErrorActionPreference = 'Stop'
$projectDir = Split-Path -Parent $PSScriptRoot
Push-Location $projectDir
try {
  go run ./cmd/baoan-policy-collector export-raw --data-dir $DataDir --output-dir $OutputDir
  if ($LASTEXITCODE -ne 0) { throw "canonical raw export failed with exit code $LASTEXITCODE" }
} finally {
  Pop-Location
}
