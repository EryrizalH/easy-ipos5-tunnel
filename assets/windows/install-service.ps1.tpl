$ErrorActionPreference = 'Stop'

function Ensure-Admin {
    $currentIdentity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($currentIdentity)
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)) {
        Write-Host "Menjalankan ulang dengan hak Administrator..." -ForegroundColor Yellow
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
    throw "nssm.exe tidak ditemukan pada folder bundle"
}

function Remove-ExistingServiceIfAny {
    param(
        [string]$ServiceName
    )

    $existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($null -eq $existing) {
        return
    }

    Write-Host "Service lama ditemukan ($ServiceName), menyiapkan reinstall..." -ForegroundColor Yellow

    Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
    & sc.exe delete $ServiceName | Out-Null

    # Tunggu sampai service benar-benar hilang agar install ulang tidak bentrok.
    $deadline = (Get-Date).AddSeconds(10)
    while ((Get-Date) -lt $deadline) {
        $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
        if ($null -eq $svc) {
            return
        }
        Start-Sleep -Milliseconds 500
    }

    throw "Service lama $ServiceName belum terhapus sepenuhnya. Tutup Services/Event Viewer yang sedang membuka service lalu jalankan ulang installer."
}

Ensure-Admin

$baseDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ratholeExe = $null
$exeCandidates = @("ipos5-rathole.exe", "rathole.exe")
foreach ($exeName in $exeCandidates) {
    $candidate = Join-Path $baseDir $exeName
    if (Test-Path $candidate) {
        $ratholeExe = $candidate
        break
    }
}

$configFile = Join-Path $baseDir "client.toml"
$serviceName = "{{WINDOWS_SERVICE_NAME}}"
$guiSetupScript = Join-Path $baseDir "install-gui-autostart.ps1"
$logRoot = Join-Path $env:ProgramData "easy-rathole-client\\logs"
$stdoutLog = Join-Path $logRoot ($serviceName + ".stdout.log")
$stderrLog = Join-Path $logRoot ($serviceName + ".stderr.log")

if (-not $ratholeExe) {
    throw "Tidak menemukan ipos5-rathole.exe atau rathole.exe pada folder bundle"
}

if (-not (Test-Path $configFile)) {
    throw "client.toml tidak ditemukan pada folder bundle"
}
if (-not (Test-Path $guiSetupScript)) {
    throw "install-gui-autostart.ps1 tidak ditemukan pada folder bundle"
}
$nssm = Ensure-Nssm -BaseDir $baseDir
Remove-ExistingServiceIfAny -ServiceName $serviceName

New-Item -ItemType Directory -Path $logRoot -Force | Out-Null

& $nssm install $serviceName $ratholeExe $configFile
& $nssm set $serviceName AppDirectory $baseDir
& $nssm set $serviceName Start SERVICE_AUTO_START
& $nssm set $serviceName DisplayName "IPOS5TunnelPublik Client"
& $nssm set $serviceName Description "Auto-start tunnel client untuk akses publik"
& $nssm set $serviceName AppStdout $stdoutLog
& $nssm set $serviceName AppStderr $stderrLog
& $nssm set $serviceName AppRotateFiles 1
& $nssm set $serviceName AppRotateOnline 1
& $nssm set $serviceName AppRotateSeconds 86400
& $nssm set $serviceName AppRotateBytes 1048576

Start-Service -Name $serviceName
Start-Sleep -Seconds 1

$svc = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
if ($null -eq $svc) {
    throw "Service $serviceName gagal dibuat"
}

if ($svc.Status -ne 'Running') {
    throw "Service $serviceName berhasil dibuat tetapi belum berjalan. Cek Windows Event Viewer dan pastikan endpoint client pada client.toml siap (default: 5444/5480/5485) serta PGbouncer aktif."
}

Write-Host "Menyiapkan GUI autostart..." -ForegroundColor Cyan
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $guiSetupScript
if ($LASTEXITCODE -ne 0) {
    throw "Setup GUI autostart gagal dengan exit code $LASTEXITCODE"
}

Write-Host "Executable terpakai: $(Split-Path -Leaf $ratholeExe)" -ForegroundColor Cyan
Write-Host "GUI executable: {{WINDOWS_GUI_BINARY_NAME}}" -ForegroundColor Cyan
Write-Host "Task autostart GUI: {{WINDOWS_GUI_TASK_NAME}}" -ForegroundColor Cyan
Write-Host "Log stderr: $stderrLog" -ForegroundColor Cyan
Write-Host "Service $serviceName berhasil diinstal dan berjalan." -ForegroundColor Green
