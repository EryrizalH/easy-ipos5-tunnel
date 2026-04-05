$ErrorActionPreference = 'Continue'

$currentIdentity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($currentIdentity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)) {
    Write-Host "Re-launching uninstall with Administrator privileges..." -ForegroundColor Yellow
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

if (Get-Service -Name $serviceName -ErrorAction SilentlyContinue) {
    Stop-Service -Name $serviceName -Force -ErrorAction SilentlyContinue
    sc.exe delete $serviceName | Out-Null
    Write-Host "Service $serviceName removed." -ForegroundColor Green
} else {
    Write-Host "Service $serviceName does not exist." -ForegroundColor Yellow
}
