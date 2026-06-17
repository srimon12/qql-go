param(
    [Parameter(Mandatory=$true)] [string]$Artifacts
)

$ErrorActionPreference = "Stop"

function Assert-True {
    param(
        [bool]$Condition,
        [string]$Message
    )

    if (-not $Condition) {
        throw $Message
    }
}

function Read-Json {
    param([string]$Path)
    return Get-Content -Raw -Path $Path | ConvertFrom-Json
}

$doctor = Read-Json (Join-Path $Artifacts "01-doctor.json")
Assert-True ($doctor.ok -and $doctor.healthy) "doctor should report a healthy connection"

$inspect = Read-Json (Join-Path $Artifacts "03-inspect.json")
Assert-True ($inspect.ok -and $inspect.data.topology -eq "hybrid") "collection should be hybrid"
Assert-True ($inspect.data.payload_schema.team.type -eq "keyword") "team payload index should remain keyword"
Assert-True ($inspect.data.payload_schema.title.type -eq "text") "title payload index should remain text"

$explain = Read-Json (Join-Path $Artifacts "04-explain.json")
Assert-True ($explain.ok -and $explain.plan.Contains("Using: HYBRID")) "explain plan should stay on the hybrid path"

$hybrid = Read-Json (Join-Path $Artifacts "05-search-hybrid.json")
Assert-True ($hybrid.ok -and $hybrid.data[0].id -eq "4104") "hybrid search should rank the billing regression incident first"

$exact = Read-Json (Join-Path $Artifacts "06-search-exact.json")
Assert-True ($exact.ok -and $exact.data[0].id -eq "4104") "exact search should rank the billing regression incident first"

$sparse = Read-Json (Join-Path $Artifacts "07-search-sparse.json")
Assert-True ($sparse.ok -and $sparse.data[0].id -eq "4104") "sparse search should rank the billing regression incident first"

$filtered = Read-Json (Join-Path $Artifacts "08-search-filtered.json")
Assert-True ($filtered.ok -and $filtered.data[0].id -eq "4104") "filtered billing search should keep the regression incident first"

$select = Read-Json (Join-Path $Artifacts "09-select-doc.json")
Assert-True $select.ok "expected document should be fetchable by ID"

$scroll = Read-Json (Join-Path $Artifacts "10-scroll-runbooks.json")
Assert-True ($scroll.ok -and $scroll.data.points.Count -ge 1) "runbook slice should be scrollable"

Write-Host "Validated retrieval debug runbook artifacts in $Artifacts" -ForegroundColor Green
