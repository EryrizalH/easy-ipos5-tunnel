param([string]$SourceDir = "")

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$guiDir = Join-Path $repoRoot "client-gui"
if (-not [string]::IsNullOrWhiteSpace($SourceDir)) {
    $guiDir = (Resolve-Path -LiteralPath $SourceDir).Path
}
$outPath = Join-Path $repoRoot "assets/windows/nusatunnel-gui.exe"
$iconPath = Join-Path $guiDir "icon.ico"
$sysoPath = Join-Path $guiDir "rsrc.syso"
$guiVerifier = Join-Path $PSScriptRoot "verify_windows_gui_cli.ps1"

if (-not (Test-Path $guiDir)) {
    throw "Folder client-gui tidak ditemukan: $guiDir"
}
if (-not (Test-Path $iconPath)) {
    throw "File icon.ico tidak ditemukan: $iconPath"
}
foreach ($name in @("index.html", "app.js", "style.css")) {
    $frontendPath = Join-Path $guiDir ("frontend/dist/" + $name)
    if (-not (Test-Path -LiteralPath $frontendPath -PathType Leaf)) {
        throw "Frontend GUI belum lengkap: $frontendPath. Siapkan source frontend sebelum build."
    }
}

Write-Host "Generating Windows resource (.syso) from icon..." -ForegroundColor Cyan
& go install github.com/akavel/rsrc@latest
if ($LASTEXITCODE -ne 0) {
    throw "Gagal install tool rsrc"
}

$goBin = (& go env GOPATH).Trim()
if (-not $goBin) {
    throw "Gagal membaca GOPATH"
}
$rsrcExe = Join-Path $goBin "bin\\rsrc.exe"
if (-not (Test-Path $rsrcExe)) {
    throw "rsrc.exe tidak ditemukan setelah install: $rsrcExe"
}

Push-Location $guiDir
try {
    & $rsrcExe -ico $iconPath -o $sysoPath
    if ($LASTEXITCODE -ne 0) {
        throw "Gagal generate rsrc.syso dari icon.ico"
    }
}
finally {
    Pop-Location
}

Write-Host "Building GUI with Wails production tag..." -ForegroundColor Cyan
Push-Location $guiDir
try {
    & go build -tags production -ldflags "-H=windowsgui" -o $outPath .
    if ($LASTEXITCODE -ne 0) {
        throw "go build gagal dengan exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}

if (-not (Test-Path $outPath)) {
    throw "Output GUI exe tidak ditemukan: $outPath"
}

$file = Get-Item $outPath
if ($file.Length -le 0) {
    throw "Output GUI exe kosong: $outPath"
}

& $guiVerifier -GUIPath $outPath

Write-Host "Sukses build GUI: $($file.FullName) ($($file.Length) bytes)" -ForegroundColor Green
