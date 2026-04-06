$ErrorActionPreference = 'Continue'

$currentIdentity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($currentIdentity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)) {
    Write-Host "Menjalankan ulang uninstall dengan hak Administrator..." -ForegroundColor Yellow
    $argList = @(
        '-NoProfile',
        '-ExecutionPolicy',
        'Bypass',
        '-File',
        '"' + $PSCommandPath + '"'
    )
    Start-Process -FilePath 'powershell.exe' -Verb RunAs -ArgumentList ($argList -join ' ')
    exit 0
}

$serviceName = "{{WINDOWS_SERVICE_NAME}}"
$guiUninstallScript = Join-Path (Split-Path -Parent $MyInvocation.MyCommand.Path) "uninstall-gui-autostart.ps1"

if (Get-Service -Name $serviceName -ErrorAction SilentlyContinue) {
    Stop-Service -Name $serviceName -Force -ErrorAction SilentlyContinue
    sc.exe delete $serviceName | Out-Null
    Write-Host "Service $serviceName berhasil dihapus." -ForegroundColor Green
} else {
    Write-Host "Service $serviceName tidak ditemukan." -ForegroundColor Yellow
}

if (Test-Path $guiUninstallScript) {
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $guiUninstallScript
} else {
    Write-Host "Script uninstall GUI autostart tidak ditemukan, skip." -ForegroundColor Yellow
}
