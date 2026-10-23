param(
    [Parameter(Mandatory = $true)][string]$GUIPath
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path -LiteralPath $GUIPath -PathType Leaf)) {
    throw "GUI tidak ditemukan: $GUIPath. Jalankan scripts/build_windows_gui.ps1 terlebih dahulu."
}

$guiExecutable = (Resolve-Path -LiteralPath $GUIPath).Path
$probeDir = Join-Path ([System.IO.Path]::GetTempPath()) ("nusatunnel-gui-check-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $probeDir | Out-Null
$stdoutPath = Join-Path $probeDir "stdout.txt"
$stderrPath = Join-Path $probeDir "stderr.txt"
$process = $null
try {
    # --help stops flag parsing before the GUI, tray or config store starts.
    # Supplying --config first also detects older GUIs that reject the shortcut's argument.
    $process = Start-Process -FilePath $guiExecutable -ArgumentList @("--config=probe-client.toml", "--help") -WindowStyle Hidden -PassThru -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath
    # Windows PowerShell 5 can lose ExitCode when a short-lived process exits
    # before its handle is retained. Keep it open before waiting.
    $null = $process.Handle
    if (-not $process.WaitForExit(10000)) {
        throw "Pemeriksaan CLI GUI melewati batas waktu. Build ulang dengan scripts/build_windows_gui.ps1."
    }
    $helpText = (Get-Content -LiteralPath $stdoutPath -Raw) + (Get-Content -LiteralPath $stderrPath -Raw)
    if ($process.ExitCode -ne 0 -or $helpText -notmatch '(?m)^\s+-{1,2}config(?:[=\s]|$)') {
        throw "GUI belum mendukung --config yang dipakai shortcut desktop. Jalankan scripts/build_windows_gui.ps1 sebelum build setup.exe."
    }
}
finally {
    if ($null -ne $process) {
        if (-not $process.HasExited) {
            $process.Kill()
            $process.WaitForExit()
        }
        $process.Dispose()
    }
    Remove-Item -LiteralPath $stdoutPath, $stderrPath -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $probeDir -Force
}

Write-Host "CLI GUI mendukung argumen shortcut --config." -ForegroundColor Green
