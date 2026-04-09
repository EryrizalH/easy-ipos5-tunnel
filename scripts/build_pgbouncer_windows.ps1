$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$sourceDir = Join-Path $repoRoot "tmp\pgbouncer-src"
$assetsDir = Join-Path $repoRoot "assets\windows"
$msysBash = "C:\msys64\usr\bin\bash.exe"

if (-not (Test-Path $msysBash)) {
    throw "MSYS2 bash tidak ditemukan di $msysBash. Install dulu: winget install --id MSYS2.MSYS2 -e"
}

if (-not (Test-Path $sourceDir)) {
    & git clone https://github.com/pgbouncer/pgbouncer $sourceDir
    if ($LASTEXITCODE -ne 0) {
        throw "Gagal clone repo pgbouncer"
    }
}

$repoRootForMsys = $repoRoot -replace '\\', '/'
$buildCmd = @"
set -euo pipefail
export PATH=/mingw64/bin:/usr/bin:$PATH
repo_root=\$(cygpath -u "$repoRootForMsys")
cd "\$repo_root/tmp/pgbouncer-src"
./autogen.sh
CC=x86_64-w64-mingw32-gcc ./configure --host=x86_64-w64-mingw32
make -j4 pgbouncer.exe pgbevent.dll
"@

& $msysBash -lc $buildCmd
if ($LASTEXITCODE -ne 0) {
    throw "Build pgbouncer gagal"
}

$files = @(
    @{ Src = (Join-Path $sourceDir "pgbouncer.exe"); Dst = (Join-Path $assetsDir "pgbouncer.exe") },
    @{ Src = "C:\msys64\mingw64\bin\libevent-7.dll"; Dst = (Join-Path $assetsDir "libevent-7.dll") },
    @{ Src = "C:\msys64\mingw64\bin\libssl-3-x64.dll"; Dst = (Join-Path $assetsDir "libssl-3-x64.dll") },
    @{ Src = "C:\msys64\mingw64\bin\libcrypto-3-x64.dll"; Dst = (Join-Path $assetsDir "libcrypto-3-x64.dll") }
)

foreach ($file in $files) {
    if (-not (Test-Path $file.Src)) {
        throw "File hasil build/dependency tidak ditemukan: $($file.Src)"
    }
    Copy-Item $file.Src $file.Dst -Force
}

Write-Host "Sukses build dan salin asset PGbouncer ke $assetsDir" -ForegroundColor Green
