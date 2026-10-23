param([string]$GUISourceDir = "")

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$unifiedDir = Join-Path $repoRoot "lock-ipos"
$outPath = Join-Path $repoRoot "assets/windows/setup.exe"
$guiBuilder = Join-Path $PSScriptRoot "build_windows_gui.ps1"
$payloadBuilder = Join-Path $PSScriptRoot "build_windows_installer_payload.ps1"
$stagingPath = Join-Path (Split-Path -Parent $outPath) ("setup-build-" + [guid]::NewGuid().ToString("N") + ".exe")

if (-not (Test-Path $unifiedDir)) {
    throw "Folder lock-ipos tidak ditemukan: $unifiedDir"
}
if (-not (Test-Path $payloadBuilder)) {
    throw "Builder payload installer tidak ditemukan: $payloadBuilder"
}
if (-not (Test-Path $guiBuilder)) {
    throw "Builder GUI tidak ditemukan: $guiBuilder"
}

Write-Host "Building interactive Windows setup.exe..." -ForegroundColor Cyan

$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "1"

# Shortcut setup.exe passes --config; always package a GUI built from the same source.
& $guiBuilder -SourceDir $GUISourceDir
if ($LASTEXITCODE -ne 0) {
    throw "Gagal membangun GUI sebelum menggabungkan payload installer"
}

try {
    Push-Location $unifiedDir
    try {
        & go build -ldflags "-s -w" -o $stagingPath .
        if ($LASTEXITCODE -ne 0) {
            throw "go build setup.exe gagal dengan exit code $LASTEXITCODE"
        }
    }
    finally {
        Pop-Location
    }

    if (-not (Test-Path -LiteralPath $stagingPath)) {
        throw "Output setup.exe tidak ditemukan: $stagingPath"
    }

    & $payloadBuilder -SetupPath $stagingPath -AssetsDir (Join-Path $repoRoot "assets/windows") -IncludeGUI
    if ($LASTEXITCODE -ne 0) {
        throw "Gagal menggabungkan payload runtime ke setup.exe"
    }

    if ((Get-Item -LiteralPath $stagingPath).Length -le 0) {
        throw "Output setup.exe kosong: $stagingPath"
    }
    # Publish only after the complete runtime payload has been appended.
    Move-Item -LiteralPath $stagingPath -Destination $outPath -Force
}
finally {
    if (Test-Path -LiteralPath $stagingPath) {
        Remove-Item -LiteralPath $stagingPath -Force
    }
}

$file = Get-Item -LiteralPath $outPath
Write-Host "Sukses build setup.exe: $($file.FullName) ($($file.Length) bytes)" -ForegroundColor Green
