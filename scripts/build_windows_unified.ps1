$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$unifiedDir = Join-Path $repoRoot "lock-ipos"
$outPath = Join-Path $repoRoot "assets/windows/setup.exe"
$payloadBuilder = Join-Path $PSScriptRoot "build_windows_installer_payload.ps1"

if (-not (Test-Path $unifiedDir)) {
    throw "Folder lock-ipos tidak ditemukan: $unifiedDir"
}
if (-not (Test-Path $payloadBuilder)) {
    throw "Builder payload installer tidak ditemukan: $payloadBuilder"
}

Write-Host "Building interactive Windows setup.exe..." -ForegroundColor Cyan

$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "1"

Push-Location $unifiedDir
try {
    & go build -ldflags "-s -w" -o $outPath .
    if ($LASTEXITCODE -ne 0) {
        throw "go build setup.exe gagal dengan exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}

if (-not (Test-Path $outPath)) {
    throw "Output setup.exe tidak ditemukan: $outPath"
}

& $payloadBuilder -SetupPath $outPath -AssetsDir (Join-Path $repoRoot "assets/windows") -IncludeGUI
if ($LASTEXITCODE -ne 0) {
    throw "Gagal menggabungkan payload runtime ke setup.exe"
}

$file = Get-Item $outPath
if ($file.Length -le 0) {
    throw "Output setup.exe kosong: $outPath"
}

Write-Host "Sukses build setup.exe: $($file.FullName) ($($file.Length) bytes)" -ForegroundColor Green
