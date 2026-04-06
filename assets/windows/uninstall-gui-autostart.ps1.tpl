$ErrorActionPreference = 'Continue'

$taskName = "{{WINDOWS_GUI_TASK_NAME}}"

& schtasks.exe /Delete /TN $taskName /F | Out-Null
if ($LASTEXITCODE -eq 0) {
    Write-Host "Autostart GUI task dihapus: $taskName" -ForegroundColor Green
} else {
    Write-Host "Autostart GUI task tidak ditemukan atau gagal dihapus: $taskName" -ForegroundColor Yellow
}

