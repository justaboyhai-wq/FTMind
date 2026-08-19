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
  $run = Get-Content -LiteralPath $manifest.FullName -Raw -Encoding UTF8 | ConvertFrom-Json
  if ([string]::IsNullOrWhiteSpace($run.run_id)) { throw "Invalid run manifest: $($manifest.FullName)" }
  $verified++
}

$checksumFiles = @(Get-ChildItem -LiteralPath (Join-Path $DataDir "policies") -Filter "checksums.sha256" -Recurse -File -ErrorAction SilentlyContinue)
$filesVerified = 0
foreach ($checksumFile in $checksumFiles) {
  foreach ($line in Get-Content -LiteralPath $checksumFile.FullName) {
    if ([string]::IsNullOrWhiteSpace($line)) { continue }
    $parts = $line -split "  ", 2
    if ($parts.Count -ne 2) { throw "Invalid checksum line in $($checksumFile.FullName)" }
    $target = Join-Path $checksumFile.DirectoryName $parts[1]
    if (-not (Test-Path -LiteralPath $target -PathType Leaf)) { throw "Missing archived file: $target" }
    $actual = (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $parts[0].ToLowerInvariant()) { throw "Checksum mismatch: $target" }
    $filesVerified++
  }
}
Write-Output "Verified $verified run manifest(s), $filesVerified archived file checksum(s)."
