# AI Development Workflow

This guide is for AI-assisted development in this repo. Use it before implementing non-trivial changes so the dashboard, installer, bundles, and Windows tools stay aligned.

## Initialization Checklist

Run these checks before changing code:

```powershell
git status --short
rg --files
```

Read the root `AGENTS.md`, the project skill, and any nested instruction files for the affected folders. `AGENTS.md` is the canonical filename; do not create a competing `AGENT.md`. Preserve existing changes, including deleted frontend files and modified binary assets.

Check available runtimes from the repo root:

```powershell
python --version
go version
Test-Path .venv/Scripts/python.exe
Test-Path client-gui/frontend/dist/index.html
```

- Dashboard requires Python 3.11+ because it imports `datetime.UTC`. The Ubuntu 22+ OS check alone does not guarantee this runtime; the installer currently invokes the target's `python3`.
- Modern Go development requires Go 1.23+ (`client-gui/go.mod`); `lock-ipos/go.mod` targets Go 1.20. Windows 7 releases specifically use Go 1.20.14 at `%USERPROFILE%\go\bin\go1.20.14.exe`.
- GUI tests/builds embed the tracked `client-gui/frontend/dist` files. There is currently no `package.json` or `wails.json` frontend generation workflow. If those files are locally deleted, report the prerequisite failure and preserve the deletions.
- If `go` is unavailable, report Go verification as not run; installing a system toolchain is separate from preparing project dependencies.

If dashboard dependencies are missing, create `.venv` only when it does not already exist, then install the pinned project requirements:

```powershell
python -m venv .venv
.\.venv\Scripts\python.exe -m pip install -r dashboard\requirements.txt
```

When Go is available, download module dependencies without rewriting `go.mod`/`go.sum` with `go mod tidy`:

```powershell
Push-Location client-gui
go mod download
Pop-Location
Push-Location lock-ipos
go mod download
Pop-Location
```

Do not run `install.sh`, `public-install.sh`, or `scripts/prepare_server.sh` locally as a quick test. They are designed for root execution on Ubuntu 22+ and can change firewall, SSH, systemd, and package state.

## Routing By Change Type

### Dashboard

Touch `dashboard/app/*` when changing:

- Basic auth, token rotation, dashboard display, `/health`, `/download/*`, PostgreSQL monitor, bundle generation.
- State paths and resources controlled by `NUSA_TUNNEL_*` env vars.

Guardrails:

- Keep tests isolated with temp dirs and env overrides.
- If bundle files change, update dashboard instructions/error hints and operator docs. Current bundles do not contain a generated README.
- If service status changes, keep Linux service names aligned with installer state.

Verification:

```powershell
Push-Location dashboard
..\.venv\Scripts\python.exe -m unittest discover -s app
Pop-Location
```

### Installer And Rathole Templates

Touch `install.sh`, `public-install.sh`, `scripts/*`, and `templates/rathole/*` when changing:

- Ubuntu install flow, hardening, rathole release download, systemd service, firewall, state file, server/client TOML rendering.

Guardrails:

- Preserve Ubuntu 22+ requirement unless explicitly changed.
- Preserve idempotent state updates in `/opt/nusatunnel/state/install-state.json`.
- If `service_ports` changes, update firewall, server template rendering, dashboard display, client bundle rendering, and tests.
- Keep `curl | bash` flow simple for public install.

Verification:

- Static shell review is acceptable for narrow edits.
- Full install verification should run only on a disposable Ubuntu 22+ VM/VPS.
- Check expected runtime commands in `docs/OPERATIONS.md`.

### Windows Setup / lock-ipos

Touch `lock-ipos/*` when changing:

- Interactive setup menu, PgBouncer/Pengoptimal Database install/uninstall, PostgreSQL lock/allow behavior, service wrapper, installer progress.

Guardrails:

- Lock/allow semantics affect PostgreSQL ownership/permissions and should not break ordinary client access.
- PgBouncer mode must keep `0.0.0.0:5444` as the exposed listener and PostgreSQL on `127.0.0.1:5445`.
- Installer copy should make consequences visible before execution.
- Install IP Public tunes PostgreSQL before installing the tunnel: backup the active config, preserve unrelated settings/ports, restart only when effective values differ, and verify via a fresh SQL connection. Failures restore the changed files; a failed restart also triggers service recovery.
- Resolve the selected PostgreSQL instance from Windows service/postmaster metadata; do not guess a global data path. Tuning uses 25% RAM bounded to 128–512 MB for shared buffers and Windows-compatible keepalive count `0`.
- The tuning block persists after tunnel uninstall. Windows 7 binaries are updated separately; source changes do not refresh that packaged installer automatically.

Verification:

```powershell
Push-Location lock-ipos
go test ./...
Pop-Location
```

### Client GUI

Touch `client-gui/*` when changing:

- Wails app lifecycle, tray actions, status snapshots, service control, auth failure detection, config path, auto-start.

Guardrails:

- Close button hides to tray unless quitting.
- Service control targets `NusaTunnelClient`.
- Keep admin/UAC expectations visible in user-facing messages.

Verification:

```powershell
Push-Location client-gui
go test ./...
Pop-Location
```

### Windows Bundles And Assets

Touch `assets/windows*` and build scripts only when changing packaging or refreshing binaries.

Guardrails:

- Modern Windows bundle contains exactly `setup.exe` dan `client.toml`; runtime tunnel, GUI, NSSM, dan PgBouncer ada di dalam `setup.exe`.
- Windows 7 bundle juga berisi tepat dua file dan payload-nya sengaja tanpa `nusatunnel-gui.exe`.
- Do not alter binary assets accidentally during doc or source-only changes.

Build commands, only for an explicitly requested binary refresh. Build the wrapper before unified setup: `build_windows_unified.ps1` rebuilds the GUI and embeds the runtime in `assets/windows`. The Windows 7 script builds its own setup and wrapper before embedding assets from `assets/windows7`:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build_windows_service_wrapper.ps1
powershell -ExecutionPolicy Bypass -File scripts/build_windows_unified.ps1
powershell -ExecutionPolicy Bypass -File scripts/build_windows7_assets.ps1
```

Build modern `build_windows_unified.ps1` otomatis membangun ulang GUI dari source sebelum menggabungkan payload. Builder payload memeriksa output `nusatunnel-gui.exe --help` untuk memastikan argumen `--config` tersedia; GUI lama dapat langsung keluar saat dibuka lewat shortcut yang memakai argumen tersebut. Setelah build, gunakan `setup.exe` yang baru dan jalankan ulang instalasi pada client agar runtime terkelola ikut diperbarui.

Tes packaging memakai fixture CLI Go di folder sementara, tanpa menjalankan instalasi atau mengubah asset repo:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/test_windows_installer_payload.ps1
```
Payload packaging also requires the runtime files listed in `scripts/build_windows_installer_payload.ps1` (NSSM, tunnel, PgBouncer and DLLs). The GUI script uses `go build -tags production`; it does not require the Wails CLI. None of these build commands is part of routine workflow initialization.

`scripts/verify_windows_gui_cli.ps1 -GUIPath assets/windows/nusatunnel-gui.exe` checks the CLI with `--help` without opening the application. Windows 7 has no GUI and does not need this check. To build from a complete staging copy while preserving local frontend deletions, pass `-GUISourceDir <staged-client-gui>` to the unified builder (or `-SourceDir` to the standalone GUI builder).

## Cross-Surface Checklist

Use this checklist when a change touches ports, services, state, tokens, or bundles:

- Dashboard routes and display updated.
- Installer state JSON updated or migration-safe.
- Rathole server/client templates updated.
- Firewall opening still covers remote bind ports and dashboard port.
- Windows and Windows 7 bundle generation produces exactly `setup.exe` dan `client.toml`, dengan payload internal sesuai varian.
- Operator docs and dashboard instructions/error messages still match actual bundle contents.
- Tests cover normalization, rendering, or packaging behavior.
- Final response states exactly what passed and what remains unverified.

## Current Default Contract

- Server OS: Ubuntu 22+.
- Dashboard port: initially `8088`, configurable with `DASHBOARD_PORT` during server installation, then persisted as `dashboard_port` in install state and rendered into the systemd Uvicorn command. `NUSA_TUNNEL_PORT` is not consumed by the current installer.
- Server services: `rathole`, `nusatunnel-dashboard`.
- Client services: `nusatunnel-client` on Linux, `NusaTunnelClient` on Windows.
- PgBouncer service: `NusaTunnelDB`.
- Default rathole control port: `443`.
- Default remote ports: `5444`, `5480`, `5485`.
- State: `/opt/nusatunnel/state/install-state.json`.
- Server config: `/etc/nusatunnel/server.toml`.
- Dashboard DB: `/opt/nusatunnel/state/nusatunnel.db`.
- Bundle output: `/opt/nusatunnel/bundles`.

## GitHub Release distribution

Deployment is built from a tag, not a Git checkout. Read `docs/RELEASE.md` when editing installer/package behavior. The allowlist and dependency lock are in `scripts/release.py` and `scripts/release-dependencies.json`. Generated setup/GUI/wrapper/SYSO files are ignored; vendored third-party Windows runtime remains tracked. GUI frontend `dist` remains source and must not be ignored or restored automatically.

Release Windows builds generate ignored Go credentials from `IPOS5_DB_USERNAME` and `IPOS5_DB_PASSWORD`; production release fails if either is unavailable. Do not print values, put them on command lines, cache generated build outputs, or pass secrets to PR workflows. Public Windows installers still embed those credentials intentionally. Python deployment uses a root-only monitor environment file collected via `/dev/tty`, never build-time defaults.

Run both dashboard discovery commands: `python -m unittest discover -s app` and `python -m unittest discover -s app/services` from `dashboard`. Run `python -m unittest discover -s tests` from root for release/HTTP/bootstrap/credential tests. Unix terminal and permission tests require Linux. Full root installer scripts still require a disposable target.
