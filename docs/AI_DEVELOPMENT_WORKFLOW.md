# AI Development Workflow

This guide is for AI-assisted development in this repo. Use it before implementing non-trivial changes so the dashboard, installer, bundles, and Windows tools stay aligned.

## Initialization Checklist

Run these checks before changing code:

```powershell
git status --short
rg --files
```

If dependencies are missing:

```powershell
python -m venv .venv
.\.venv\Scripts\pip.exe install -r dashboard\requirements.txt
```

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
- If bundle files change, update error hints and generated README text.
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

- Modern Windows bundle includes `setup.exe`, `nusatunnel-service.exe`, `nusatunnel.exe`, `nusatunnel-gui.exe`, `nssm.exe`, PgBouncer binary/libs, `client.toml`, `pgbouncer.ini`, `pgbouncer-databases.json`, and `userlist.sample.txt`.
- Windows 7 bundle excludes `nusatunnel-gui.exe`.
- Do not alter binary assets accidentally during doc or source-only changes.

Build commands:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build_windows_unified.ps1
powershell -ExecutionPolicy Bypass -File scripts/build_windows_service_wrapper.ps1
powershell -ExecutionPolicy Bypass -File scripts/build_windows_gui.ps1
powershell -ExecutionPolicy Bypass -File scripts/build_windows7_assets.ps1
```

## Cross-Surface Checklist

Use this checklist when a change touches ports, services, state, tokens, or bundles:

- Dashboard routes and display updated.
- Installer state JSON updated or migration-safe.
- Rathole server/client templates updated.
- Firewall opening still covers remote bind ports and dashboard port.
- Windows and Windows 7 bundle generation still match required assets.
- Generated README/error messages still match actual bundle contents.
- Tests cover normalization, rendering, or packaging behavior.
- Final response states exactly what passed and what remains unverified.

## Current Default Contract

- Server OS: Ubuntu 22+.
- Dashboard port: `8088` unless `NUSA_TUNNEL_PORT` overrides it.
- Server services: `rathole`, `nusatunnel-dashboard`.
- Client services: `nusatunnel-client` on Linux, `NusaTunnelClient` on Windows.
- PgBouncer service: `NusaTunnelDB`.
- Default rathole control port: `443`.
- Default remote ports: `5444`, `5480`, `5485`.
- State: `/opt/nusatunnel/state/install-state.json`.
- Server config: `/etc/nusatunnel/server.toml`.
- Dashboard DB: `/opt/nusatunnel/state/nusatunnel.db`.
- Bundle output: `/opt/nusatunnel/bundles`.

