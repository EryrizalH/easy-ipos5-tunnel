param([string]$GoCommand = "go")

$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem

$builder = Join-Path $PSScriptRoot "build_windows_installer_payload.ps1"
$testDir = Join-Path ([System.IO.Path]::GetTempPath()) ("nusatunnel payload test " + [System.Guid]::NewGuid().ToString())
$savedGOOS = $env:GOOS
$savedGOARCH = $env:GOARCH
$savedCGO = $env:CGO_ENABLED

function Assert-True([bool]$Condition, [string]$Message) {
    if (-not $Condition) { throw $Message }
}

function Build-GUIFixture([bool]$WithConfig) {
    $source = @'
package main
import "flag"
func main() {
    flag.Bool("hidden", false, "start window hidden")
    // CONFIG_FLAG
    flag.Parse()
}
'@
    if ($WithConfig) {
        $source = $source.Replace("// CONFIG_FLAG", 'flag.String("config", "", "override path to client.toml")')
    }
    $sourcePath = Join-Path $testDir "gui.go"
    [System.IO.File]::WriteAllText($sourcePath, $source)
    $buildArgs = @("build")
    if ($env:GOOS -eq "windows") {
        $buildArgs += @("-ldflags", "-H=windowsgui")
    }
    & $GoCommand @buildArgs -o $guiPath $sourcePath
    Assert-True ($LASTEXITCODE -eq 0) "Gagal membangun fixture GUI"
}

function Assert-Payload([string]$Path, [string[]]$ExpectedNames) {
    # ZIP offsets are relative to the payload; .NET's reader needs the prefix removed.
    $bytes = [System.IO.File]::ReadAllBytes($Path)
    $prefixLength = [System.Text.Encoding]::UTF8.GetByteCount($setupStub)
    Assert-True ([System.Text.Encoding]::UTF8.GetString($bytes, 0, $prefixLength) -eq $setupStub) "Awalan installer berubah"
    Assert-True ($bytes.Length -gt $prefixLength) "Payload tidak ditambahkan"
    $zipPath = Join-Path $testDir "payload.zip"
    [System.IO.File]::WriteAllBytes($zipPath, [byte[]]$bytes[$prefixLength..($bytes.Length - 1)])
    $archive = [System.IO.Compression.ZipFile]::OpenRead($zipPath)
    try {
        $names = @($archive.Entries | ForEach-Object { $_.FullName } | Sort-Object)
        Assert-True (($names -join ",") -eq (($ExpectedNames | Sort-Object) -join ",")) "Isi payload tidak sesuai. Aktual: $($names -join ','); diharapkan: $($ExpectedNames -join ',')"
        foreach ($entry in $archive.Entries) {
            Assert-True ($entry.Length -gt 0) "Payload kosong: $($entry.FullName)"
        }
    }
    finally {
        $archive.Dispose()
    }
}

try {
    New-Item -ItemType Directory -Path $testDir | Out-Null
    $hostPlatform = @(& $GoCommand env GOHOSTOS GOHOSTARCH)
    Assert-True ($LASTEXITCODE -eq 0 -and $hostPlatform.Count -eq 2) "Gagal membaca platform Go"
    $env:GOOS = $hostPlatform[0]
    $env:GOARCH = $hostPlatform[1]
    $env:CGO_ENABLED = "0"

    $runtimeFiles = @(
        "nssm.exe", "nusatunnel-service.exe", "nusatunnel.exe", "pgbouncer.exe",
        "libevent-7.dll", "libssl-3-x64.dll", "libcrypto-3-x64.dll", "libwinpthread-1.dll"
    )
    foreach ($name in $runtimeFiles) {
        [System.IO.File]::WriteAllText((Join-Path $testDir $name), "runtime fixture")
    }
    $guiPath = Join-Path $testDir "nusatunnel-gui.exe"
    $setupPath = Join-Path $testDir "setup.exe"
    $setupStub = "MZ installer fixture"
    [System.IO.File]::WriteAllText($setupPath, $setupStub)

    Build-GUIFixture $false
    $rejected = $false
    try {
        & $builder -SetupPath $setupPath -AssetsDir $testDir -IncludeGUI
    }
    catch {
        if ($_.Exception.Message -notlike "*GUI belum mendukung --config*") { throw }
        $rejected = $true
    }
    Assert-True $rejected "GUI lama harus ditolak"
    Assert-True ([System.IO.File]::ReadAllText($setupPath) -eq $setupStub) "Installer berubah setelah GUI ditolak"

    Build-GUIFixture $true
    & $builder -SetupPath $setupPath -AssetsDir $testDir -IncludeGUI
    Assert-Payload $setupPath ($runtimeFiles + "nusatunnel-gui.exe")

    Remove-Item $guiPath
    [System.IO.File]::WriteAllText($setupPath, $setupStub)
    & $builder -SetupPath $setupPath -AssetsDir $testDir
    Assert-Payload $setupPath $runtimeFiles

    Write-Host "PASS: GUI lama ditolak tanpa mengubah installer; payload modern dan Windows 7 valid." -ForegroundColor Green
}
finally {
    $env:GOOS = $savedGOOS
    $env:GOARCH = $savedGOARCH
    $env:CGO_ENABLED = $savedCGO
    $resolvedTestDir = [System.IO.Path]::GetFullPath($testDir)
    $tempRoot = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath()).TrimEnd('\', '/')
    if ((Split-Path -Parent $resolvedTestDir).TrimEnd('\', '/') -ne $tempRoot -or (Split-Path -Leaf $resolvedTestDir) -notlike 'nusatunnel payload test *') {
        throw "Lokasi cleanup fixture tidak valid: $resolvedTestDir"
    }
    Remove-Item -LiteralPath $resolvedTestDir -Recurse -Force -ErrorAction SilentlyContinue
}
