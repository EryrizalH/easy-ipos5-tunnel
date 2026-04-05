$ErrorActionPreference = 'Continue'

$serviceName = "{{WINDOWS_SERVICE_NAME}}"

if (Get-Service -Name $serviceName -ErrorAction SilentlyContinue) {
    Stop-Service -Name $serviceName -Force -ErrorAction SilentlyContinue
    sc.exe delete $serviceName | Out-Null
    Write-Host "Service $serviceName removed." -ForegroundColor Green
} else {
    Write-Host "Service $serviceName does not exist." -ForegroundColor Yellow
}
