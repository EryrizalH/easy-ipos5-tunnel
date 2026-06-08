$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$lockIposDir = Join-Path $repoRoot "lock-ipos"
$serviceDir = Join-Path $lockIposDir "cmd\ipos5-rathole-service"
$assets7Dir = Join-Path $repoRoot "assets\windows7"

$go120 = Join-Path $env:USERPROFILE "go\bin\go1.20.14.exe"

if (-not (Test-Path $go120)) {
    throw "Go 1.20.14 compiler tidak ditemukan di $go120. Silakan install dengan running: go install golang.org/dl/go1.20.14@latest; & `"\$env:USERPROFILE/go/bin/go1.20.14`" download"
}

if (-not (Test-Path $lockIposDir)) {
    throw "Folder lock-ipos tidak ditemukan: $lockIposDir"
}

if (-not (Test-Path $assets7Dir)) {
    New-Item -ItemType Directory -Path $assets7Dir -Force | Out-Null
}

$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "1"

# 1. Build setup.exe untuk Windows 7
$setupOut = Join-Path $assets7Dir "setup.exe"
Write-Host "Building Windows 7 setup.exe..." -ForegroundColor Cyan
Push-Location $lockIposDir
try {
    & $go120 build -ldflags "-s -w" -o $setupOut .
    if ($LASTEXITCODE -ne 0) {
        throw "go build setup.exe gagal"
    }
}
finally {
    Pop-Location
}

# 2. Build service wrapper untuk Windows 7
$serviceOut = Join-Path $assets7Dir "ipos5-rathole-service.exe"
Write-Host "Building Windows 7 service wrapper..." -ForegroundColor Cyan
Push-Location $serviceDir
try {
    & $go120 build -ldflags "-s -w" -o $serviceOut .
    if ($LASTEXITCODE -ne 0) {
        throw "go build service wrapper gagal"
    }
}
finally {
    Pop-Location
}

Write-Host "Sukses build asset Windows 7 ke $assets7Dir" -ForegroundColor Green
