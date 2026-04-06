@echo off
setlocal

echo [IPOS5TunnelPublik] Menginstal service client Windows...
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0install-service.ps1"
set "rc=%ERRORLEVEL%"

if not "%rc%"=="0" (
  echo.
  echo [ERROR] Instalasi gagal dengan kode %rc%.
  echo Pastikan Anda klik kanan file ini dan pilih "Run as administrator".
  pause
  exit /b %rc%
)

echo.
echo [OK] Instalasi selesai.
pause
exit /b 0
