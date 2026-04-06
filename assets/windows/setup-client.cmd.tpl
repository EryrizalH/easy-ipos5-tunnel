@echo off
setlocal EnableExtensions

title IPOS5TunnelPublik Windows Client Setup

echo ==================================================
echo   IPOS5TunnelPublik - Setup Client Windows Sekali Klik
echo ==================================================
echo.

:: Auto re-launch as Administrator if needed
powershell.exe -NoProfile -Command "$p = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent()); if ($p.IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)) { exit 0 } else { exit 1 }"
if not "%ERRORLEVEL%"=="0" (
  echo [INFO] Script butuh hak Administrator. Meminta izin UAC...
  powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
  exit /b 0
)

cd /d "%~dp0"

echo [CHECK] Verifikasi file bundle...
if not exist "client.toml" (
  echo [ERROR] File client.toml tidak ditemukan.
  echo         Pastikan ZIP sudah di-extract penuh lalu jalankan ulang script ini.
  pause
  exit /b 1
)

set "RATHOLE_BIN="
if exist "ipos5-rathole.exe" set "RATHOLE_BIN=ipos5-rathole.exe"
if not defined RATHOLE_BIN if exist "rathole.exe" set "RATHOLE_BIN=rathole.exe"

if not defined RATHOLE_BIN (
  echo [ERROR] Binary rathole tidak ditemukan.
  echo         Cari salah satu file ini di folder yang sama:
  echo         - ipos5-rathole.exe
  echo         - rathole.exe
  pause
  exit /b 1
)

echo [OK] Binary ditemukan: %RATHOLE_BIN%

if not exist "{{WINDOWS_GUI_BINARY_NAME}}" (
  echo [ERROR] GUI binary tidak ditemukan: {{WINDOWS_GUI_BINARY_NAME}}
  echo         Pastikan ZIP client windows diextract lengkap.
  pause
  exit /b 1
)

echo [OK] GUI binary ditemukan: {{WINDOWS_GUI_BINARY_NAME}}
echo.
echo [STEP 1/2] Install service Windows...
call "%~dp0install-service.cmd"
if not "%ERRORLEVEL%"=="0" (
  echo.
  echo [ERROR] Install service gagal.
  pause
  exit /b 1
)

echo.
echo [STEP 2/2] Verifikasi service...
sc query "{{WINDOWS_SERVICE_NAME}}" | findstr /I "RUNNING" >nul
if not "%ERRORLEVEL%"=="0" (
  echo [WARN] Service belum status RUNNING.
  echo        Silakan cek Event Viewer jika diperlukan.
) else (
  echo [OK] Service {{WINDOWS_SERVICE_NAME}} aktif (RUNNING).
)

echo [CHECK] Verifikasi task auto-start GUI...
schtasks /Query /TN "{{WINDOWS_GUI_TASK_NAME}}" >nul 2>&1
if not "%ERRORLEVEL%"=="0" (
  echo [WARN] Task auto-start GUI tidak ditemukan: {{WINDOWS_GUI_TASK_NAME}}
  echo        Silakan jalankan ulang setup-client.cmd sebagai Administrator.
) else (
  echo [OK] Task auto-start GUI terdaftar: {{WINDOWS_GUI_TASK_NAME}}
)

echo [CHECK] Verifikasi GUI berjalan (tray)...
tasklist /FI "IMAGENAME eq {{WINDOWS_GUI_BINARY_NAME}}" | findstr /I "{{WINDOWS_GUI_BINARY_NAME}}" >nul
if not "%ERRORLEVEL%"=="0" (
  echo [WARN] GUI belum terdeteksi berjalan.
  echo        Coba buka manual {{WINDOWS_GUI_BINARY_NAME}} sekali, lalu cek icon tray.
) else (
  echo [OK] GUI terdeteksi berjalan. Icon tray seharusnya muncul.
)

echo.
echo Selesai.
echo Jika aplikasi POS lokal Anda aktif di port 5444/5480/5485,
echo tunnel publik akan langsung bekerja.
echo.
pause
exit /b 0
