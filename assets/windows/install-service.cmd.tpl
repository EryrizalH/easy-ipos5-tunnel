@echo off
setlocal

echo [Easy Rathole] Installing Windows client service...
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0install-service.ps1"
set "rc=%ERRORLEVEL%"

if not "%rc%"=="0" (
  echo.
  echo [ERROR] Install failed with code %rc%.
  echo Pastikan Anda klik kanan file ini dan pilih "Run as administrator".
  pause
  exit /b %rc%
)

echo.
echo [OK] Install selesai.
pause
exit /b 0
