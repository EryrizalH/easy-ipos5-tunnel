$ErrorActionPreference = 'Stop'

function Wait-GuiProcess {
    param(
        [string]$GuiExe,
        [int]$TimeoutSeconds = 10
    )

    $exeName = Split-Path -Leaf $GuiExe
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)

    while ((Get-Date) -lt $deadline) {
        $running = Get-CimInstance Win32_Process -Filter ("Name='{0}'" -f $exeName) -ErrorAction SilentlyContinue |
            Where-Object { $_.ExecutablePath -and ($_.ExecutablePath -ieq $GuiExe) }

        if ($running) {
            return $true
        }

        Start-Sleep -Milliseconds 500
    }

    return $false
}

$baseDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$guiExe = Join-Path $baseDir "{{WINDOWS_GUI_BINARY_NAME}}"
$taskName = "{{WINDOWS_GUI_TASK_NAME}}"

if (-not (Test-Path $guiExe)) {
    throw "GUI executable tidak ditemukan: {{WINDOWS_GUI_BINARY_NAME}}"
}

# Hapus Mark-of-the-Web bila ada agar tidak memicu prompt keamanan saat auto-run.
try {
    Unblock-File -Path $guiExe -ErrorAction Stop
} catch {
    # Abaikan jika file memang tidak terblokir atau platform tidak mendukung ADS.
}

$taskCommand = '"' + $guiExe + '" --hidden'

& schtasks.exe /Create /TN $taskName /TR $taskCommand /SC ONLOGON /RL HIGHEST /F | Out-Null
if ($LASTEXITCODE -ne 0) {
    throw "Gagal membuat scheduled task $taskName"
}

& schtasks.exe /Query /TN $taskName | Out-Null
if ($LASTEXITCODE -ne 0) {
    throw "Scheduled task $taskName tidak bisa diverifikasi setelah dibuat"
}

$guiStarted = $false

# Coba jalankan lewat Task Scheduler agar behavior sama persis dengan auto-start saat logon.
& schtasks.exe /Run /TN $taskName | Out-Null
if ($LASTEXITCODE -eq 0) {
    $guiStarted = Wait-GuiProcess -GuiExe $guiExe -TimeoutSeconds 10
}

# Fallback: jalankan langsung bila pemicu task gagal/terlambat.
if (-not $guiStarted) {
    try {
        Start-Process -FilePath $guiExe -ArgumentList "--hidden" -WindowStyle Hidden -ErrorAction Stop | Out-Null
    } catch {
        Write-Warning "Start-Process GUI gagal: $($_.Exception.Message)"
    }
    $guiStarted = Wait-GuiProcess -GuiExe $guiExe -TimeoutSeconds 10
}

if (-not $guiStarted) {
    throw "GUI belum berhasil berjalan. Jalankan {{WINDOWS_GUI_BINARY_NAME}} sekali secara manual (Allow/Run jika ada prompt keamanan), lalu ulangi setup-client.cmd."
}

Write-Host "Autostart GUI aktif: $taskName" -ForegroundColor Green
Write-Host "GUI berjalan (hidden) dan icon tray siap digunakan." -ForegroundColor Green

