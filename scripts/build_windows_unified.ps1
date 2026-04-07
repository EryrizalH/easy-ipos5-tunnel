$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$unifiedDir = Join-Path $repoRoot "lock-ipos"
$outPath = Join-Path $repoRoot "assets/windows/setup.exe"

if (-not (Test-Path $unifiedDir)) {
    throw "Folder lock-ipos tidak ditemukan: $unifiedDir"
}

Write-Host "Building unified Windows client..." -ForegroundColor Cyan

$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "1"

Push-Location $unifiedDir
try {
    & go build -ldflags "-s -w" -o $outPath .
    if ($LASTEXITCODE -ne 0) {
        throw "go build unified gagal dengan exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}

if (-not (Test-Path $outPath)) {
    throw "Output unified exe tidak ditemukan: $outPath"
}

$file = Get-Item $outPath
if ($file.Length -le 0) {
    throw "Output unified exe kosong: $outPath"
}

Write-Host "Sukses build unified exe: $($file.FullName) ($($file.Length) bytes)" -ForegroundColor Green
