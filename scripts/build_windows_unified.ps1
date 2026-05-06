$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$unifiedDir = Join-Path $repoRoot "lock-ipos"
$outPath = Join-Path $repoRoot "assets/windows/setup.exe"

if (-not (Test-Path $unifiedDir)) {
    throw "Folder lock-ipos tidak ditemukan: $unifiedDir"
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

$file = Get-Item $outPath
if ($file.Length -le 0) {
    throw "Output setup.exe kosong: $outPath"
}

Write-Host "Sukses build setup.exe: $($file.FullName) ($($file.Length) bytes)" -ForegroundColor Green
