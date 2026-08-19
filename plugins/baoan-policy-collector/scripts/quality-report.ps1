param(
  [Parameter(Mandatory = $true)]
  [string]$DataDir,
  [string]$OutFile
)

$ErrorActionPreference = "Stop"
$policyDirs = @(Get-ChildItem -LiteralPath (Join-Path $DataDir "policies") -Directory -ErrorAction SilentlyContinue)
$report = [ordered]@{
  generated_at = [DateTime]::UtcNow.ToString("o")
  policy_count = 0
  dimensions = [ordered]@{ service_objects = @{}; authorities = @{}; themes = @{}; carriers = @{}; application_status = @{} }
  relation_codes = @{}
  attachments = [ordered]@{ declared = 0; archived = 0; missing = 0 }
  conflicts = 0
  unknown_official_values = 0
}

function Add-Count([hashtable]$Table, [string]$Value) {
  if ([string]::IsNullOrWhiteSpace($Value)) { $Value = "<unknown>" }
  if (-not $Table.ContainsKey($Value)) { $Table[$Value] = 0 }
  $Table[$Value]++
}

foreach ($policy in $policyDirs) {
  $latestPath = Join-Path $policy.FullName "latest.json"
  if (-not (Test-Path -LiteralPath $latestPath)) { continue }
  $latest = Get-Content -LiteralPath $latestPath -Raw -Encoding UTF8 | ConvertFrom-Json
  $snapshot = Join-Path $policy.FullName (Join-Path "snapshots" $latest.snapshot_id)
  $structuredPath = Join-Path $snapshot "structured.json"
  $manifestPath = Join-Path $snapshot "manifest.json"
  if (-not (Test-Path -LiteralPath $structuredPath)) { continue }
  $structured = Get-Content -LiteralPath $structuredPath -Raw -Encoding UTF8 | ConvertFrom-Json
  $report.policy_count++
  foreach ($x in @($structured.official.service_objects)) { Add-Count $report.dimensions.service_objects ([string]$x) }
  Add-Count $report.dimensions.authorities ([string]$structured.official.issuing_authority)
  Add-Count $report.dimensions.themes ([string]$structured.official.theme)
  Add-Count $report.dimensions.carriers ([string]$structured.official.carrier_type)
  Add-Count $report.dimensions.application_status ([string]$structured.local_application_status)
  if (@($structured.conflicts).Count -gt 0) { $report.conflicts += @($structured.conflicts).Count }
  if ([string]::IsNullOrWhiteSpace([string]$structured.official.issuing_authority) -or [string]::IsNullOrWhiteSpace([string]$structured.official.theme)) { $report.unknown_official_values++ }
  if (Test-Path -LiteralPath $manifestPath) {
    $manifest = Get-Content -LiteralPath $manifestPath -Raw -Encoding UTF8 | ConvertFrom-Json
    foreach ($a in @($manifest.attachments)) {
      $report.attachments.declared++
      $attachmentPath = Join-Path $snapshot ([string]$a.storage_path)
      if (Test-Path -LiteralPath $attachmentPath -PathType Leaf) { $report.attachments.archived++ } else { $report.attachments.missing++ }
    }
  }
  $relationsPath = Join-Path $snapshot "relations.json"
  if (Test-Path -LiteralPath $relationsPath) {
    $relations = Get-Content -LiteralPath $relationsPath -Raw -Encoding UTF8 | ConvertFrom-Json
    foreach ($relation in $relations) {
      Add-Count $report.relation_codes ([string]$relation.relation_type)
    }
  }
}

$json = $report | ConvertTo-Json -Depth 8
if ($OutFile) { Set-Content -LiteralPath $OutFile -Value $json -Encoding UTF8 }
$json
