param(
    [Parameter(Mandatory = $true)][string]$SetupPath,
    [Parameter(Mandatory = $true)][string]$AssetsDir,
    [switch]$IncludeGUI
)

$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.IO.Compression.FileSystem

$payloadFiles = @(
    "nssm.exe",
    "nusatunnel-service.exe",
    "nusatunnel.exe",
    "pgbouncer.exe",
    "libevent-7.dll",
    "libssl-3-x64.dll",
    "libcrypto-3-x64.dll",
    "libwinpthread-1.dll"
)
if ($IncludeGUI) {
    $payloadFiles += "nusatunnel-gui.exe"
}

foreach ($name in $payloadFiles) {
    $path = Join-Path $AssetsDir $name
    if (-not (Test-Path -Path $path -PathType Leaf)) {
        throw "Asset payload wajib tidak ditemukan: $path"
    }
    if ((Get-Item $path).Length -le 0) {
        throw "Asset payload kosong: $path"
    }
}

$tempZip = Join-Path ([System.IO.Path]::GetTempPath()) ("nusatunnel-payload-" + [System.Guid]::NewGuid().ToString() + ".zip")
try {
    $zipStream = [System.IO.File]::Open($tempZip, [System.IO.FileMode]::CreateNew, [System.IO.FileAccess]::Write)
    try {
        $archive = [System.IO.Compression.ZipArchive]::new($zipStream, [System.IO.Compression.ZipArchiveMode]::Create, $false)
        try {
            foreach ($name in $payloadFiles) {
                $entry = $archive.CreateEntry($name, [System.IO.Compression.CompressionLevel]::Optimal)
                $entryStream = $entry.Open()
                $sourceStream = [System.IO.File]::OpenRead((Join-Path $AssetsDir $name))
                try {
                    $sourceStream.CopyTo($entryStream)
                }
                finally {
                    $sourceStream.Dispose()
                    $entryStream.Dispose()
                }
            }
        }
        finally {
            $archive.Dispose()
        }
    }
    finally {
        $zipStream.Dispose()
    }

    $zipInput = [System.IO.File]::OpenRead($tempZip)
    $setupOutput = [System.IO.File]::Open($SetupPath, [System.IO.FileMode]::Append, [System.IO.FileAccess]::Write)
    try {
        $zipInput.CopyTo($setupOutput)
    }
    finally {
        $setupOutput.Dispose()
        $zipInput.Dispose()
    }
}
finally {
    Remove-Item -Force -ErrorAction SilentlyContinue $tempZip
}

Write-Host "Payload runtime berhasil digabungkan ke $SetupPath" -ForegroundColor Green
