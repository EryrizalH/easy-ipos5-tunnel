@echo off
setlocal

echo [IPOS5TunnelPublik] Menghapus service client Windows...
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0uninstall-service.ps1"
set "rc=%ERRORLEVEL%"

if not "%rc%"=="0" (
  echo.
  echo [WARN] Uninstall exited with code %rc%.
  pause
  exit /b %rc%
)

echo.
echo [OK] Proses uninstall selesai.
pause
exit /b 0
