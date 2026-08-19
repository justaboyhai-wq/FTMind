param(
  [switch]$AllowPublish,
  [string]$Version = "2.0.0"
)

$ErrorActionPreference = "Stop"

if (-not $AllowPublish) {
  throw "Publishing changes external ClawHub state. Re-run with -AllowPublish after confirming the logged-in ClawHub account and version."
}

$clawhub = Get-Command clawhub -ErrorAction SilentlyContinue
if (-not $clawhub) {
  throw "The clawhub CLI is not installed. Install it with 'npm i -g clawhub', then run 'clawhub login'."
}

$skillDir = Join-Path $PSScriptRoot "..\frontend\public\fmind-skill"
if (-not (Test-Path (Join-Path $skillDir "SKILL.md"))) {
  throw "FMind skill package is missing SKILL.md: $skillDir"
}

& $clawhub.Source skill publish $skillDir --slug fmind --version $Version
if ($LASTEXITCODE -ne 0) {
  throw "ClawHub publish failed with exit code $LASTEXITCODE. No repository files were changed by this script."
}
