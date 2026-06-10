---
name: easy-rathole-workflow
description: Guide development and verification for the easy-Rathole project, including FastAPI dashboard changes, Ubuntu Bash installer work, rathole templates, Go/Wails client GUI, lock-ipos Windows setup, PgBouncer integration, and Windows/Windows 7 bundle packaging.
---

# Easy Rathole Workflow

Use this skill before implementing changes in `D:\Koding\easy-Rathole`.

## Required First Steps

1. Read `AGENTS.md`.
2. Check `git status --short` and preserve unrelated changes.
3. Read the relevant files for the changed surface.
4. Use `docs/AI_DEVELOPMENT_WORKFLOW.md` for the detailed command matrix and cross-surface checklist.

## Project Surfaces

- `dashboard/`: FastAPI dashboard, SQLite state, Basic Auth, token rotation, PostgreSQL monitor, bundle download endpoints.
- `install.sh`, `public-install.sh`, `scripts/`, `templates/`: Ubuntu 22+ install flow, hardening, systemd, firewall, rathole TOML.
- `client-gui/`: Go/Wails GUI, tray lifecycle, service control, auth failure detection.
- `lock-ipos/`: Windows interactive setup, PgBouncer install/uninstall, PostgreSQL lock/allow behavior, service wrapper.
- `assets/windows` and `assets/windows7`: packaged runtime assets and generated binaries.

## Invariants

- Preserve remote ports `5444`, `5480`, and `5485` unless explicitly changed.
- Preserve DB routing: VPS `5444` -> rathole client `127.0.0.1:5444`; PgBouncer mode uses `0.0.0.0:5444` -> PostgreSQL `127.0.0.1:5445`.
- Preserve service names: `rathole`, `easy-rathole-dashboard`, `easy-rathole-client`, `EasyRatholeClient`, and `PgBouncer`.
- Keep Windows 7 bundle separate and without `ipos5-rathole-gui.exe`.
- Do not run root/system-changing installer scripts locally unless explicitly requested for a clear Ubuntu target.
- Do not modify binary assets unless the task is a build or bundle refresh.
- Do not expose real tokens, dashboard credentials, or PgBouncer auth files.

## Verification

Dashboard:

```powershell
Push-Location dashboard
..\.venv\Scripts\python.exe -m unittest discover -s app
Pop-Location
```

Client GUI:

```powershell
Push-Location client-gui
go test ./...
Pop-Location
```

lock-ipos:

```powershell
Push-Location lock-ipos
go test ./...
Pop-Location
```

Windows builds, only when requested:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build_windows_unified.ps1
powershell -ExecutionPolicy Bypass -File scripts/build_windows_service_wrapper.ps1
powershell -ExecutionPolicy Bypass -File scripts/build_windows_gui.ps1
powershell -ExecutionPolicy Bypass -File scripts/build_windows7_assets.ps1
```

Always report what was verified and what remains untested because it requires Ubuntu, Windows service execution, or a real tunnel.
