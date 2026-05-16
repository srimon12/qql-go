# demo-lib.ps1 - Fluent engine for qql-go demos

$ErrorActionPreference = "Stop"

# --- INTERNAL HELPERS ---

function Resolve-QqlBinary {
    param([string]$RepoRoot)
    $localBinary = Join-Path $RepoRoot "qql-go.exe"
    if (Test-Path $localBinary) { return $localBinary }
    $cmd = Get-Command qql-go -ErrorAction SilentlyContinue
    if ($null -ne $cmd) { return $cmd.Source }
    throw "Could not find qql-go binary."
}

function Initialize-DemoArtifacts {
    param([string]$DemoRoot)
    $artifactDir = Join-Path $DemoRoot "artifacts"
    if (Test-Path $artifactDir) { Remove-Item -Recurse -Force $artifactDir }
    New-Item -ItemType Directory -Path $artifactDir | Out-Null
    return $artifactDir
}

# --- FLUENT API (User Facing) ---

function Setup-Demo {
    # Get caller's directory
    $scriptPath = (Get-Variable MyInvocation -Scope 1).Value.MyCommand.Path
    $global:DEMO_ROOT = Split-Path -Parent $scriptPath
    $global:REPO_ROOT = Resolve-Path (Join-Path $global:DEMO_ROOT "..\..")
    $global:QQL = Resolve-QqlBinary -RepoRoot $global:REPO_ROOT
    $global:ARTIFACTS = Initialize-DemoArtifacts -DemoRoot $global:DEMO_ROOT
}

function Step {
    param(
        [Parameter(Mandatory=$true, Position=0)] [string]$Id,
        [Parameter(Mandatory=$true, Position=1)] [string]$Command,
        [Parameter(ValueFromRemainingArguments=$true)] [string[]]$Args
    )

    $artifactPath = Join-Path $global:ARTIFACTS "$Id.json"
    $allArgs = @($Command, "--quiet", "--json") + $Args
    $raw = & $global:QQL @allArgs
    $raw | Set-Content -Path $artifactPath -Encoding utf8

    $json = $raw | ConvertFrom-Json
    if ($json.ok -eq $false) { throw "Step '$Id' failed: $($json.error)" }
}

function Reset {
    param([string]$CollectionName)
    & $global:QQL exec --quiet --json "DROP COLLECTION $CollectionName" | Out-Null
}

function Doctor {
    Step -Id "01-doctor" -Command "doctor"
}

function Finish {
    Write-Host "`n✓ Workflow complete. Artifacts saved to: $($global:ARTIFACTS)" -ForegroundColor Green
}
