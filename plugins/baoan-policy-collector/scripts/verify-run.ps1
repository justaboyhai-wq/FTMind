param(
  [Parameter(Mandatory = $true)]
  [string]$DataDir
)

$ErrorActionPreference = "Stop"
$runsRoot = Join-Path $DataDir "runs"
$manifestFiles = @(Get-ChildItem -LiteralPath $runsRoot -Filter "run-manifest.json" -Recurse -File)
if ($manifestFiles.Count -eq 0) { throw "No run manifests found under $DataDir" }

$verified = 0
foreach ($manifest in $manifestFiles) {
  $run = Get-Content -LiteralPath $manifest.FullName -Raw | ConvertFrom-Json
  if ([string]::IsNullOrWhiteSpace($run.run_id)) { throw "Invalid run manifest: $($manifest.FullName)" }
  $verified++
}
Write-Output "Verified $verified run manifest(s)."
