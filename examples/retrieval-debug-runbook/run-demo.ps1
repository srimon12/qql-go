$ErrorActionPreference = "Stop"

if (-not (Get-Command qql-go -ErrorAction SilentlyContinue)) {
    throw "qql-go must be installed and available on PATH"
}

$artifacts = if ($env:QQL_RUNBOOK_ARTIFACTS) { $env:QQL_RUNBOOK_ARTIFACTS } else { Join-Path $PSScriptRoot "artifacts" }
$collection = "retrieval_debug_runbook"

if (Test-Path $artifacts) {
    Remove-Item -Recurse -Force $artifacts
}
New-Item -ItemType Directory -Path $artifacts | Out-Null

function Run-Step {
    param(
        [string]$Id,
        [string]$Command,
        [string]$Statement
    )

    $artifact = Join-Path $artifacts "$Id.json"
    $raw = & qql-go $Command --quiet --json $Statement
    $raw | Set-Content -Path $artifact -Encoding utf8
    $json = $raw | ConvertFrom-Json
    if (-not $json.ok) {
        throw "Step '$Id' failed: $raw"
    }
}

Write-Host "Running retrieval debug runbook..."

(& qql-go doctor --quiet --json) | Set-Content -Path (Join-Path $artifacts "01-doctor.json") -Encoding utf8
& qql-go exec --quiet --json "DROP COLLECTION $collection" | Out-Null
(& qql-go execute --quiet --json (Join-Path $PSScriptRoot "01-seed.qql")) | Set-Content -Path (Join-Path $artifacts "02-seed.json") -Encoding utf8

Run-Step "03-inspect" "exec" "SHOW COLLECTION $collection"
Run-Step "04-explain" "explain" "QUERY 'billing policy search regression after index removal' FROM $collection LIMIT 3 USING HYBRID"
Run-Step "05-search-hybrid" "exec" "QUERY 'billing policy search regression after index removal' FROM $collection LIMIT 3 USING HYBRID"
Run-Step "06-search-exact" "exec" "QUERY 'billing policy search regression after index removal' FROM $collection LIMIT 3 EXACT"
Run-Step "07-search-sparse" "exec" "QUERY 'billing policy search regression after index removal' FROM $collection LIMIT 3 USING SPARSE"
Run-Step "08-search-filtered" "exec" "QUERY 'billing policy search regression after index removal' FROM $collection LIMIT 3 USING HYBRID WHERE team = 'billing'"
Run-Step "09-select-doc" "exec" "SELECT * FROM $collection WHERE id = 4104"
Run-Step "10-scroll-runbooks" "exec" "SCROLL FROM $collection WHERE doc_type = 'runbook' LIMIT 10"

& (Join-Path $PSScriptRoot "validate-artifacts.ps1") -Artifacts $artifacts

Write-Host "Workflow complete. Artifacts saved to: $artifacts"
