$ErrorActionPreference = 'Stop'

$baseDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$guiExe = Join-Path $baseDir "{{WINDOWS_GUI_BINARY_NAME}}"
$taskName = "{{WINDOWS_GUI_TASK_NAME}}"

if (-not (Test-Path $guiExe)) {
    throw "GUI executable tidak ditemukan: {{WINDOWS_GUI_BINARY_NAME}}"
}

$taskCommand = '"' + $guiExe + '" --hidden'

& schtasks.exe /Create /TN $taskName /TR $taskCommand /SC ONLOGON /F | Out-Null
if ($LASTEXITCODE -ne 0) {
    throw "Gagal membuat scheduled task $taskName"
}

# Jalankan sekali setelah install agar user langsung mendapat tray app.
Start-Process -FilePath $guiExe -ArgumentList "--hidden" -WindowStyle Hidden | Out-Null

Write-Host "Autostart GUI aktif: $taskName" -ForegroundColor Green

