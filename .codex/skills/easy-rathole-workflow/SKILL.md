---
name: easy-rathole-workflow
description: Develop and verify the D:\Koding\easy-Rathole project. Use when working on the FastAPI dashboard, Bash Ubuntu installer, rathole server/client templates, Go/Wails client GUI, lock-ipos Windows installer/TUI, PgBouncer integration, Windows or Windows 7 client bundles, service names, port mappings, or project AI workflow/guideline updates.
---

# Easy Rathole Workflow

Use this skill to avoid changing one deployment surface while forgetting the others. This repo ships an Ubuntu server installer, a FastAPI dashboard, generated client bundles, Windows services, a Go/Wails GUI, and a Go TUI installer.

## Start Here

1. Read `AGENTS.md` first.
2. Check `git status --short` and preserve unrelated user changes.
3. Identify the touched surface:
   - `dashboard/`: FastAPI, SQLite state, token rotation, bundle download, PostgreSQL monitor.
   - `scripts/`, `templates/`, `install.sh`, `public-install.sh`: Ubuntu 22+ server install and systemd resources.
   - `client-gui/`: Go/Wails Windows GUI and tray behavior.
   - `lock-ipos/`: interactive Windows setup flow, PgBouncer install, service wrapper, PostgreSQL lock/allow behavior.
   - `assets/windows` and `assets/windows7`: generated or bundled binaries and runtime files.
4. Use `docs/AI_DEVELOPMENT_WORKFLOW.md` for the detailed checklist and command matrix.

## Invariants

- Keep default remote ports `5444`, `5480`, and `5485` unless the user explicitly changes routing.
- Keep the DB tunnel mapping coherent: VPS `5444` -> rathole client `127.0.0.1:5444`; with PgBouncer, PgBouncer listens on `0.0.0.0:5444` and PostgreSQL moves to `127.0.0.1:5445`.
- Preserve service names unless the change is explicitly about renaming services: Linux server `rathole`, dashboard `easy-rathole-dashboard`, Linux client `easy-rathole-client`, Windows client `EasyRatholeClient`, PgBouncer `PgBouncer`.
- Keep Windows 7 as a separate bundle path under `assets/windows7`; it intentionally does not include `ipos5-rathole-gui.exe`.
- Do not run root-level installer scripts such as `install.sh` or `scripts/prepare_server.sh` on the development machine unless the user explicitly asks and the target environment is clear.
- Do not churn binary assets in `assets/windows*` unless the task is explicitly a build or bundle refresh.
- Treat tokens, dashboard credentials, `userlist.txt`, and PgBouncer auth files as secrets. Do not print real values in summaries.

## Verification Matrix

Use the smallest command set that covers the changed surface:

```powershell
Push-Location dashboard
..\.venv\Scripts\python.exe -m unittest discover -s app
Pop-Location
```

```powershell
Push-Location client-gui
go test ./...
Pop-Location
```

```powershell
Push-Location lock-ipos
go test ./...
Pop-Location
```

For Bash installer/template edits, prefer static review and focused render/state tests. Only run full install scripts on a disposable Ubuntu 22+ target.

For Windows binary refreshes, run the relevant build script from the repo root:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build_windows_unified.ps1
powershell -ExecutionPolicy Bypass -File scripts/build_windows_service_wrapper.ps1
powershell -ExecutionPolicy Bypass -File scripts/build_windows_gui.ps1
powershell -ExecutionPolicy Bypass -File scripts/build_windows7_assets.ps1
```

## Change Discipline

- Prefer structured parsing for JSON/TOML/config generation. Avoid fragile string edits when a helper already exists.
- When changing `service_ports`, update dashboard display, server template rendering, bundle rendering, firewall opening, and tests together.
- When changing bundle contents, update dashboard error hints, generated README text, tests, and docs.
- When changing Windows service behavior, check both `lock-ipos/internal/winservice`, `lock-ipos/internal/servicewrapper`, and `client-gui/internal/appcore`.
- Keep operator-facing text concise and mostly Indonesian, matching existing README/dashboard wording.
- Report exactly which commands ran and which platform-specific flows remain unverified.
