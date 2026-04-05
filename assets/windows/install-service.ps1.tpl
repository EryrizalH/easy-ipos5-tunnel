$ErrorActionPreference = 'Stop'

function Ensure-Admin {
    $currentIdentity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($currentIdentity)
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)) {
        Write-Host "Re-launching with Administrator privileges..." -ForegroundColor Yellow
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
}

function Ensure-Nssm {
    param(
        [string]$BaseDir
    )

    $nssmPath = Join-Path $BaseDir "nssm.exe"
    if (Test-Path $nssmPath) {
        return $nssmPath
    }

    $tempZip = Join-Path $env:TEMP "nssm-2.24.zip"
    $tempDir = Join-Path $env:TEMP "nssm-2.24"
    Invoke-WebRequest -Uri "https://nssm.cc/release/nssm-2.24.zip" -OutFile $tempZip
    if (Test-Path $tempDir) {
        Remove-Item -Recurse -Force $tempDir
    }

    Expand-Archive -Path $tempZip -DestinationPath $tempDir
    $binary = Join-Path $tempDir "nssm-2.24\win64\nssm.exe"
    if (-not (Test-Path $binary)) {
        throw "Cannot locate nssm.exe in extracted archive"
    }

    Copy-Item $binary $nssmPath -Force
    return $nssmPath
}

Ensure-Admin

$baseDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ratholeExe = Join-Path $baseDir "rathole.exe"
$configFile = Join-Path $baseDir "client.toml"
$serviceName = "{{WINDOWS_SERVICE_NAME}}"

if (-not (Test-Path $ratholeExe)) {
    throw "rathole.exe is missing from bundle folder"
}

if (-not (Test-Path $configFile)) {
    throw "client.toml is missing from bundle folder"
}

$nssm = Ensure-Nssm -BaseDir $baseDir

& $nssm stop $serviceName | Out-Null
& sc.exe delete $serviceName | Out-Null

& $nssm install $serviceName $ratholeExe $configFile
& $nssm set $serviceName AppDirectory $baseDir
& $nssm set $serviceName Start SERVICE_AUTO_START
& $nssm set $serviceName DisplayName "Easy Rathole Client"
& $nssm set $serviceName Description "Auto-start rathole client tunnel"

Start-Service -Name $serviceName
Start-Sleep -Seconds 1

$svc = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
if ($null -eq $svc) {
    throw "Service $serviceName was not created"
}

if ($svc.Status -ne 'Running') {
    throw "Service $serviceName created but not running. Check Windows Event Viewer and ensure local ports 5444/5480/5485 are open on client side."
}

Write-Host "Service $serviceName installed and running." -ForegroundColor Green
