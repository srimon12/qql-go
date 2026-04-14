param(
    [string]$Version = "",
    [string]$InstallDir = "$HOME\AppData\Local\Programs\qql-go\bin"
)

$ErrorActionPreference = "Stop"

$owner = "srimon12"
$repo = "qql-go"

if ([string]::IsNullOrWhiteSpace($Version)) {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$owner/$repo/releases/latest"
    $Version = $release.tag_name
}

$Version = $Version.Trim()
$Version = $Version.TrimStart("v")

$asset = "qql-go_${Version}_windows_amd64.zip"
$baseUrl = "https://github.com/$owner/$repo/releases/download/v$Version"

$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("qql-go-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tempDir | Out-Null

try {
    $archivePath = Join-Path $tempDir $asset
    Invoke-WebRequest -Uri "$baseUrl/$asset" -OutFile $archivePath

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Expand-Archive -Path $archivePath -DestinationPath $tempDir -Force

    $binaryPath = Join-Path $InstallDir "qql-go.exe"
    Copy-Item -Path (Join-Path $tempDir "qql-go.exe") -Destination $binaryPath -Force

    Write-Host "Installed qql-go to $binaryPath"
    Write-Host "If needed, add $InstallDir to your PATH."
}
finally {
    if (Test-Path $tempDir) {
        Remove-Item -Path $tempDir -Recurse -Force
    }
}
