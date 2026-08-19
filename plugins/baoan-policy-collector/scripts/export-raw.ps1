param(
  [Parameter(Mandatory=$true)][string]$DataDir,
  [Parameter(Mandatory=$true)][string]$OutputDir
)

$ErrorActionPreference = 'Stop'
$policiesDir = Join-Path $DataDir 'policies'
if (-not (Test-Path -LiteralPath $policiesDir)) { throw "policies directory not found: $policiesDir" }
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
$count = 0
Get-ChildItem -LiteralPath $policiesDir -Directory | ForEach-Object {
  $package = $_.FullName
  $latest = Get-Content -Raw -Encoding UTF8 -LiteralPath (Join-Path $package 'latest.json') | ConvertFrom-Json
  $snapshot = Join-Path (Join-Path $package 'snapshots') $latest.snapshot_id
  $structuredPath = Join-Path $snapshot 'structured.json'
  $markdownPath = Join-Path $snapshot 'normalized.md'
  if (-not (Test-Path -LiteralPath $structuredPath) -or -not (Test-Path -LiteralPath $markdownPath)) { return }
  $doc = Get-Content -Raw -Encoding UTF8 -LiteralPath $structuredPath | ConvertFrom-Json
  $target = Join-Path $OutputDir ($_.Name + '.md')
  $meta = [ordered]@{
    schema_version = 'baoan.raw/v1'
    external_id = $_.Name
    title = $doc.title
    source_url = $doc.source_url
    final_url = $doc.final_url
    published_at = $doc.published_at
    effective_at = $doc.effective_at
    expires_at = $doc.expires_at
    official_listed = $doc.official_listed
    local_application_status = $doc.local_application_status
    official = $doc.official
    raw_package = "policies/$($_.Name)/snapshots/$($latest.snapshot_id)"
  }
  $frontMatter = ($meta | ConvertTo-Json -Depth 12)
  $body = Get-Content -Raw -Encoding UTF8 -LiteralPath $markdownPath
  Set-Content -LiteralPath $target -Value ("---`n$frontMatter`n---`n`n$body") -Encoding utf8
  Copy-Item -LiteralPath $structuredPath -Destination (Join-Path $OutputDir ($_.Name + '.structured.json')) -Force
  $count++
}
Write-Output "exported $count raw documents to $OutputDir"
