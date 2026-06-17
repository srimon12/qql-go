param(
    [Parameter(Mandatory=$true)] [string]$SuitePath,
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

$suite = Read-Json $SuitePath
$doctor = Read-Json (Join-Path $Artifacts "01-doctor.json")
Assert-True ($doctor.ok -and $doctor.healthy) "doctor should report a healthy connection"

$inspect = Read-Json (Join-Path $Artifacts "02-inspect.json")
Assert-True $inspect.ok "inspect step should succeed"
if ($suite.collection_expect.topology) {
    Assert-True ($inspect.data.topology -eq $suite.collection_expect.topology) "collection topology should stay $($suite.collection_expect.topology)"
}
if ($null -ne $suite.collection_expect.min_points) {
    Assert-True (($inspect.data.points_count) -ge $suite.collection_expect.min_points) "collection should have at least $($suite.collection_expect.min_points) points"
}
foreach ($field in $suite.collection_expect.payload_indexes) {
    Assert-True ($null -ne $inspect.data.payload_schema.$field) "payload index '$field' should exist"
}

foreach ($check in $suite.checks) {
    $artifact = Read-Json (Join-Path $Artifacts "$($check.id).json")
    Assert-True $artifact.ok "step '$($check.id)' should succeed"

    foreach ($snippet in $check.expect.plan_contains) {
        Assert-True ($artifact.plan.Contains($snippet)) "step '$($check.id)' plan should contain '$snippet'"
    }

    if ($null -ne $check.expect.min_results) {
        Assert-True (($artifact.data.Count) -ge $check.expect.min_results) "step '$($check.id)' should return at least $($check.expect.min_results) results"
    }
    if ($null -ne $check.expect.hybrid) {
        Assert-True (($artifact.data.hybrid) -eq $check.expect.hybrid) "step '$($check.id)' hybrid flag should be $($check.expect.hybrid)"
    }
    if ($check.expect.group_by) {
        Assert-True ($artifact.data.group_by -eq $check.expect.group_by) "step '$($check.id)' should stay grouped by '$($check.expect.group_by)'"
    }
    if ($check.expect.first_group) {
        Assert-True ($artifact.data.groups[0].group_id -eq $check.expect.first_group) "step '$($check.id)' should keep group '$($check.expect.first_group)' first"
    }

    for ($i = 0; $i -lt $check.expect.top_ids.Count; $i++) {
        Assert-True ($artifact.data[$i].id -eq $check.expect.top_ids[$i]) "step '$($check.id)' result[$i] should be '$($check.expect.top_ids[$i])'"
    }
}

Write-Host "Validated retrieval regression artifacts in $Artifacts" -ForegroundColor Green
