# AI Development Guidelines

This repo is a mixed deployment project for IPOS5TunnelPublik / easy-Rathole. Treat changes as cross-platform: an edit in one folder can affect Ubuntu server install, FastAPI dashboard, generated bundles, Windows services, and the Windows operator UI.

## Project Shape

- `install.sh`, `public-install.sh`, `scripts/`, `templates/`: Ubuntu 22+ server installer, hardening, systemd services, rathole server/client templates.
- `dashboard/`: FastAPI dashboard, SQLite state, token rotation, service status, PostgreSQL monitor, bundle generation.
- `client-gui/`: Go/Wails Windows GUI and tray app for `EasyRatholeClient`.
- `lock-ipos/`: Go interactive Windows setup/TUI, PgBouncer install flow, PostgreSQL lock/allow handling, service wrapper.
- `assets/windows` and `assets/windows7`: packaged Windows runtime assets. Avoid modifying binary files unless the user asks for a build or bundle refresh.

## Hard Invariants

- Default exposed remote ports stay `5444`, `5480`, and `5485` unless the user explicitly changes routing.
- DB forwarding must stay coherent: VPS `5444` -> rathole client `127.0.0.1:5444`; PgBouncer mode uses `0.0.0.0:5444` -> PostgreSQL `127.0.0.1:5445`.
- Service names are part of the external contract: `rathole`, `easy-rathole-dashboard`, `easy-rathole-client`, `EasyRatholeClient`, and `PgBouncer`.
- Windows 7 bundle is separate and intentionally excludes `ipos5-rathole-gui.exe`.
- Do not run root/system-changing installer scripts on the dev machine unless explicitly requested with a clear target environment.
- Do not expose real tokens, dashboard passwords, `userlist.txt` contents, or PgBouncer auth values.

## Workflow

1. Start with `git status --short`; do not revert unrelated user changes.
2. Read the files for the surface being changed plus any generated bundle/template counterpart.
3. Keep changes scoped. If ports, service names, bundle contents, or state keys change, update all dependent code paths and tests.
4. Prefer existing helpers such as `normalize_service_ports`, `render_template`, state helpers, and appcore services instead of ad hoc parsing.
5. Keep operator-facing copy concise and mostly Indonesian, matching the existing dashboard and README tone.
6. Run the smallest verification set that covers the change and report any platform flow that was not tested.

## Verification Commands

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

Windows setup / lock-ipos:

```powershell
Push-Location lock-ipos
go test ./...
Pop-Location
```

Windows binary refresh, only when requested:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build_windows_unified.ps1
powershell -ExecutionPolicy Bypass -File scripts/build_windows_service_wrapper.ps1
powershell -ExecutionPolicy Bypass -File scripts/build_windows_gui.ps1
powershell -ExecutionPolicy Bypass -File scripts/build_windows7_assets.ps1
```

Detailed workflow: `docs/AI_DEVELOPMENT_WORKFLOW.md`.

Project skill: `.codex/skills/easy-rathole-workflow/SKILL.md`.
