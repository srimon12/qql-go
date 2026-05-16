$ErrorActionPreference = "Stop"

if (-not (Get-Command qql-go -ErrorAction SilentlyContinue)) {
    throw "qql-go must be installed and available on PATH"
}

$suitePath = if ($env:QQL_REGRESSION_SUITE) { $env:QQL_REGRESSION_SUITE } else { Join-Path $PSScriptRoot "regression-suite.json" }
$artifacts = if ($env:QQL_REGRESSION_ARTIFACTS) { $env:QQL_REGRESSION_ARTIFACTS } else { Join-Path $PSScriptRoot "artifacts" }

if (-not (Test-Path $suitePath)) {
    throw "Could not find regression suite at $suitePath"
}

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

if ($env:QDRANT_URL) {
    $connectArgs = @("connect", "--quiet", "--json", "--url", $env:QDRANT_URL)
    if ($env:QDRANT_API_KEY) {
        $connectArgs += @("--secret", $env:QDRANT_API_KEY)
    }
    if ($env:EMBEDDING_ENDPOINT) {
        $connectArgs += @("--inference-mode", "external", "--embedding-endpoint", $env:EMBEDDING_ENDPOINT)
        if ($env:EMBEDDING_API_KEY) {
            $connectArgs += @("--embedding-key", $env:EMBEDDING_API_KEY)
        }
        if (-not $env:EMBEDDING_MODEL) {
            throw "EMBEDDING_MODEL is required when EMBEDDING_ENDPOINT is set"
        }
        $connectArgs += @("--embedding-model", $env:EMBEDDING_MODEL)
    }
    (& qql-go @connectArgs) | Set-Content -Path (Join-Path $artifacts "00-connect.json") -Encoding utf8
}

Write-Host "Running retrieval regression validation..."
(& qql-go doctor --quiet --json) | Set-Content -Path (Join-Path $artifacts "01-doctor.json") -Encoding utf8

$suite = Get-Content -Raw -Path $suitePath | ConvertFrom-Json
if (-not $suite.collection) {
    throw "suite must define a collection"
}

Run-Step "02-inspect" "exec" "SHOW COLLECTION $($suite.collection)"

foreach ($check in $suite.checks) {
    $command = if ($check.command) { $check.command } else { "exec" }
    Run-Step $check.id $command $check.statement
}

& (Join-Path $PSScriptRoot "validate-artifacts.ps1") -SuitePath $suitePath -Artifacts $artifacts

Write-Host "Workflow complete. Artifacts saved to: $artifacts"
