$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$wrapperDir = Join-Path $repoRoot "lock-ipos\cmd\ipos5-rathole-service"
$outPath = Join-Path $repoRoot "assets/windows/ipos5-rathole-service.exe"

if (-not (Test-Path $wrapperDir)) {
    throw "Folder service wrapper tidak ditemukan: $wrapperDir"
}

Write-Host "Building Windows service wrapper..." -ForegroundColor Cyan

$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "1"

Push-Location $wrapperDir
try {
    & go build -ldflags "-s -w" -o $outPath .
    if ($LASTEXITCODE -ne 0) {
        throw "go build service wrapper gagal dengan exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}

if (-not (Test-Path $outPath)) {
    throw "Output service wrapper tidak ditemukan: $outPath"
}

$file = Get-Item $outPath
if ($file.Length -le 0) {
    throw "Output service wrapper kosong: $outPath"
}

Write-Host "Sukses build service wrapper: $($file.FullName) ($($file.Length) bytes)" -ForegroundColor Green
