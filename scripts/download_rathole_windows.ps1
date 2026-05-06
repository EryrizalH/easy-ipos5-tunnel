$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$assetsDir = Join-Path $repoRoot "assets/windows"
$outPath = Join-Path $assetsDir "rathole.exe"
$apiUrl = "https://api.github.com/repos/rathole-org/rathole/releases/latest"

New-Item -ItemType Directory -Path $assetsDir -Force | Out-Null

Write-Host "Mengambil metadata release rathole terbaru..." -ForegroundColor Cyan
$release = Invoke-RestMethod -Uri $apiUrl
if (-not $release) {
    throw "Respons release rathole kosong"
}

$asset = $release.assets | Where-Object {
    $_.name -match '^rathole-x86_64-.*windows.*\.zip$'
} | Select-Object -First 1

if (-not $asset) {
    throw "Asset Windows x86_64 rathole tidak ditemukan pada release terbaru"
}

$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("easy-rathole-win-" + [guid]::NewGuid().ToString("N"))
$zipPath = Join-Path $tempRoot $asset.name
$extractDir = Join-Path $tempRoot "extract"

New-Item -ItemType Directory -Path $tempRoot -Force | Out-Null
New-Item -ItemType Directory -Path $extractDir -Force | Out-Null

try {
    Write-Host "Mengunduh $($asset.name)..." -ForegroundColor Cyan
    Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $zipPath

    Write-Host "Mengekstrak archive..." -ForegroundColor Cyan
    Expand-Archive -LiteralPath $zipPath -DestinationPath $extractDir -Force

    $binary = Get-ChildItem -Path $extractDir -Recurse -Filter "rathole.exe" | Select-Object -First 1
    if (-not $binary) {
        throw "rathole.exe tidak ditemukan di dalam archive"
    }

    Copy-Item -LiteralPath $binary.FullName -Destination $outPath -Force
}
finally {
    Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}

$file = Get-Item $outPath
if ($file.Length -le 0) {
    throw "Output rathole.exe kosong: $outPath"
}

Write-Host "Sukses simpan rathole.exe: $($file.FullName) ($($file.Length) bytes)" -ForegroundColor Green
