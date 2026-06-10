from __future__ import annotations

import os
import sqlite3
import socket
import subprocess
import threading
import time
from pathlib import Path
from typing import Any
from urllib.parse import quote_plus

from fastapi import Depends, FastAPI, Form, HTTPException, Request, status
from fastapi.responses import FileResponse, RedirectResponse
from fastapi.security import HTTPBasic, HTTPBasicCredentials
from fastapi.templating import Jinja2Templates

from .auth import verify_password
from .db import connect, ensure_postgres_monitor_table, get_postgres_monitor_latest, get_setting, get_user
from .services.bundle_service import (
    LINUX_SERVICE_NAME,
    WINDOWS_GUI_BINARY_NAME,
    WINDOWS_RATHOLE_BINARY_NAME,
    WINDOWS_SERVICE_WRAPPER_NAME,
    WINDOWS_UNIFIED_NAME,
    WINDOWS_NSSM_NAME,
    WINDOWS_SERVICE_NAME,
    generate_linux_bundle,
    generate_windows_bundle,
    generate_windows7_bundle,
)
from .services.postgres_monitor_service import PostgresMonitorWorker
from .services.tunnel_ports import exposed_ports_from_service_ports, normalize_service_ports
from .services.token_service import update_global_token
from .state import get_state_path, load_state

app = FastAPI(title="IPOS5TunnelPublik Dashboard", version="0.2.0")
security = HTTPBasic()
templates = Jinja2Templates(directory=str(Path(__file__).parent / "templates"))
postgres_monitor_worker = PostgresMonitorWorker()


def classify_flash_message(message: str) -> str:
    if not message:
        return "info"

    lowered = message.lower()
    if any(word in lowered for word in ("berhasil", "sukses", "selesai", "ok")):
        return "success"
    if any(word in lowered for word in ("gagal", "error", "tidak", "invalid", "failed")):
        return "error"
    if any(word in lowered for word in ("tapi", "menunggu", "waiting", "pending", "belum reachable")):
        return "warning"
    return "info"


def get_db() -> sqlite3.Connection:
    conn = connect()
    try:
        yield conn
    finally:
        conn.close()


def require_auth(
    credentials: HTTPBasicCredentials = Depends(security),
    conn: sqlite3.Connection = Depends(get_db),
) -> str:
    user = get_user(conn, credentials.username)
    if not user:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Unauthorized")

    if not verify_password(credentials.password, user["salt"], user["password_hash"]):
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Unauthorized")

    return str(user["username"])


def service_status(name: str) -> str:
    if not name:
        return "unknown"
    result = subprocess.run(
        ["systemctl", "is-active", name],
        check=False,
        capture_output=True,
        text=True,
    )
    return (result.stdout or "unknown").strip() or "unknown"


def extract_port(local_addr: str) -> int | None:
    if ":" not in local_addr:
        return None
    candidate = local_addr.rsplit(":", 1)[-1].strip(" ]")
    if not candidate.isdigit():
        return None
    return int(candidate)


def collect_listener_details() -> dict[int, dict[str, str]]:
    try:
        result = subprocess.run(
            ["ss", "-ltnpH"],
            check=False,
            capture_output=True,
            text=True,
        )
    except OSError:
        return {}

    if result.returncode != 0:
        return {}

    listeners: dict[int, dict[str, str]] = {}
    for line in result.stdout.splitlines():
        parts = line.split()
        if len(parts) < 4:
            continue

        local_addr = parts[3]
        port = extract_port(local_addr)
        if port is None:
            continue

        process = " ".join(parts[5:]) if len(parts) > 5 else "-"
        listeners[port] = {
            "listener": local_addr,
            "process": process or "-",
        }

    return listeners


def is_local_port_open(port: int) -> bool:
    try:
        with socket.create_connection(("127.0.0.1", port), timeout=0.25):
            return True
    except OSError:
        return False


def build_forward_port_status(exposed_ports: list[Any]) -> list[dict[str, Any]]:
    listener_map = collect_listener_details()
    details: list[dict[str, Any]] = []

    for item in exposed_ports:
        try:
            port = int(item)
        except (TypeError, ValueError):
            continue

        open_now = is_local_port_open(port)
        listener = listener_map.get(port, {})
        details.append(
            {
                "service": f"port_{port}",
                "protocol": "tcp",
                "port": port,
                "status": "active" if open_now else "inactive",
                "listener": listener.get("listener", "-"),
                "process": listener.get("process", "-"),
            }
        )

    return details


def build_supported_clients(public_ip: str, control_port: str) -> list[dict[str, str]]:
    endpoint = f"{public_ip}:{control_port}" if public_ip and control_port else "<unknown>"
    return [
        {
            "platform": "Linux",
            "architecture": "x86_64, aarch64/arm64",
            "service_name": LINUX_SERVICE_NAME,
            "delivery": "Paket ZIP (client.toml + install-client.sh)",
            "binary_source": "Script installer akan mengunduh rathole terbaru sesuai arsitektur",
            "setup_hint": "sudo ./install-client.sh",
            "remote_endpoint": endpoint,
        },
        {
            "platform": "Windows",
            "architecture": "x86_64",
            "service_name": WINDOWS_SERVICE_NAME,
            "delivery": "Paket ZIP (setup.exe + ipos5-rathole-service.exe + ipos5-rathole.exe + ipos5-rathole-gui.exe + nssm.exe + client.toml)",
            "binary_source": (
                "Bundled dari aset lokal dashboard: "
                f"{WINDOWS_UNIFIED_NAME} + {WINDOWS_SERVICE_WRAPPER_NAME} + {WINDOWS_RATHOLE_BINARY_NAME} + {WINDOWS_GUI_BINARY_NAME} + {WINDOWS_NSSM_NAME}"
            ),
            "setup_hint": "Jalankan setup.exe sebagai Administrator (install service membuat shortcut desktop GUI admin; GUI dibuka manual, tidak autostart)",
            "remote_endpoint": endpoint,
        },
        {
            "platform": "Windows 7",
            "architecture": "x86_64",
            "service_name": WINDOWS_SERVICE_NAME,
            "delivery": "Paket ZIP (setup.exe + ipos5-rathole-service.exe + ipos5-rathole.exe + nssm.exe + client.toml)",
            "binary_source": (
                "Bundled dari aset lokal dashboard versi Win7: "
                f"{WINDOWS_UNIFIED_NAME} + {WINDOWS_SERVICE_WRAPPER_NAME} + {WINDOWS_RATHOLE_BINARY_NAME} + {WINDOWS_NSSM_NAME}"
            ),
            "setup_hint": "Jalankan setup.exe sebagai Administrator (varian Win7 tanpa GUI desktop).",
            "remote_endpoint": endpoint,
        },
    ]


def build_client_tunnel_details(service_ports: list[dict[str, Any]]) -> list[dict[str, str]]:
    rows: list[dict[str, str]] = []
    for row in service_ports:
        service_key = str(row.get("service_key", "")).strip()
        protocol = str(row.get("protocol", "tcp")).strip().lower() or "tcp"
        local_addr = str(row.get("client_local_addr", "")).strip()
        try:
            remote_port = int(row.get("remote_bind_port"))
        except (TypeError, ValueError):
            continue

        rows.append(
            {
                "service": service_key,
                "protocol": protocol,
                "remote_port": str(remote_port),
                "client_local_addr": local_addr,
            }
        )
    return rows


def db_sync_status_label(db_sync_mode: Any) -> str:
    if not isinstance(db_sync_mode, dict) or db_sync_mode.get("enabled") is not True:
        return "disabled"
    if db_sync_mode.get("bucardo_configured") is True:
        return "configured"
    if db_sync_mode.get("waiting_for_client") is True:
        return "waiting-client"
    if db_sync_mode.get("initial_clone_done") is True:
        return "clone-done"
    return "pending-finalization"


def finalize_script_path(state: dict[str, Any]) -> Path:
    override = os.environ.get("EASY_RATHOLE_DB_SYNC_FINALIZE_SCRIPT", "").strip()
    if override:
        return Path(override)

    source_dir = os.environ.get("EASY_RATHOLE_SOURCE_DIR", "").strip() or str(state.get("source_dir", "")).strip()
    if source_dir:
        return Path(source_dir) / "scripts" / "install_db_sync_bucardo.sh"

    return Path("/opt/easy-rathole/src/easy-ipos5-tunnel/scripts/install_db_sync_bucardo.sh")


def summarize_command_output(result: subprocess.CompletedProcess[str], limit: int = 700) -> str:
    output = "\n".join(part.strip() for part in (result.stdout, result.stderr) if part and part.strip()).strip()
    if not output:
        return ""
    output = " ".join(output.split())
    if len(output) <= limit:
        return output
    return output[-limit:]


finalize_lock = threading.Lock()
last_finalize_attempt = 0.0
FINALIZE_COOLDOWN_SEC = 30.0


def run_db_sync_finalize(state: dict[str, Any]) -> tuple[bool, str]:
    db_sync_mode = state.get("db_sync_mode")
    if not isinstance(db_sync_mode, dict) or db_sync_mode.get("enabled") is not True:
        return False, "DB sync belum aktif. Jalankan installer dengan EASY_RATHOLE_INSTALL_DB_SYNC=1."
    if db_sync_mode.get("bucardo_configured") is True:
        return True, "DB sync sudah selesai; Bucardo sudah configured."

    if not finalize_lock.acquire(blocking=False):
        return False, "Proses finalisasi DB sync sedang berjalan."

    try:
        # Re-check in case it was updated while waiting for lock
        current_state = load_state()
        current_sync = current_state.get("db_sync_mode")
        if isinstance(current_sync, dict) and current_sync.get("bucardo_configured") is True:
            return True, "DB sync sudah selesai; Bucardo sudah configured."

        script_path = finalize_script_path(state)
        if not script_path.exists():
            return False, f"Script finalisasi DB sync tidak ditemukan: {script_path}"

        state_path = get_state_path()
        easy_root = state_path.parent.parent if state_path.parent.name == "state" else Path("/opt/easy-rathole")
        env = os.environ.copy()
        env.setdefault("EASY_RATHOLE_ROOT", str(easy_root))
        env["EASY_RATHOLE_STATE_FILE"] = str(state_path)

        try:
            result = subprocess.run(
                ["bash", str(script_path)],
                check=False,
                capture_output=True,
                text=True,
                env=env,
                timeout=int(os.environ.get("EASY_RATHOLE_DB_SYNC_FINALIZE_TIMEOUT", "3600")),
            )
        except (OSError, subprocess.TimeoutExpired) as exc:
            return False, f"Finalisasi DB sync gagal dijalankan: {exc}"

        if result.returncode != 0:
            detail = summarize_command_output(result)
            if detail:
                return False, f"Finalisasi DB sync gagal: {detail}"
            return False, f"Finalisasi DB sync gagal dengan exit code {result.returncode}."

        refreshed = load_state()
        refreshed_sync = refreshed.get("db_sync_mode")
        if isinstance(refreshed_sync, dict) and refreshed_sync.get("bucardo_configured") is True:
            return True, "Finalisasi DB sync berhasil: initial clone selesai dan Bucardo aktif."
        if isinstance(refreshed_sync, dict) and refreshed_sync.get("waiting_for_client") is True:
            return True, "Finalisasi DB sync menunggu client: private DB belum reachable via tunnel."
        return True, "Finalisasi DB sync selesai dijalankan."
    finally:
        finalize_lock.release()


def auto_finalize_callback() -> None:
    global last_finalize_attempt
    now = time.time()
    if now - last_finalize_attempt < FINALIZE_COOLDOWN_SEC:
        return

    state = load_state()
    db_sync = state.get("db_sync_mode")
    if isinstance(db_sync, dict) and db_sync.get("enabled") is True and not db_sync.get("bucardo_configured"):
        tunnel_addr = db_sync.get("private_db_tunnel_addr", "127.0.0.1:5444")
        if ":" in tunnel_addr:
            host, port_str = tunnel_addr.rsplit(":", 1)
            if port_str.isdigit():
                port = int(port_str)
                try:
                    with socket.create_connection((host, port), timeout=0.5):
                        last_finalize_attempt = now
                        run_db_sync_finalize(state)
                except OSError:
                    pass


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}


@app.get("/api/debug/logs/{service}")
def get_debug_logs(service: str, _: str = Depends(require_auth)) -> dict[str, str]:
    if service not in ("rathole", "easy-rathole-dashboard"):
        raise HTTPException(status_code=400, detail="Service tidak valid")
    try:
        result = subprocess.run(
            ["journalctl", "-u", service, "-n", "50", "--no-pager"],
            check=False,
            capture_output=True,
            text=True,
        )
        return {"service": service, "logs": result.stdout or result.stderr or ""}
    except Exception as exc:
        return {"service": service, "logs": f"Gagal mengambil log: {exc}"}


@app.get("/api/debug/bucardo")
def get_bucardo_status(_: str = Depends(require_auth)) -> dict[str, str]:
    try:
        result_status = subprocess.run(
            ["bucardo", "status"],
            check=False,
            capture_output=True,
            text=True,
        )
        result_sync = subprocess.run(
            ["bucardo", "list", "sync"],
            check=False,
            capture_output=True,
            text=True,
        )
        return {
            "status": result_status.stdout or result_status.stderr or "",
            "sync": result_sync.stdout or result_sync.stderr or "",
        }
    except Exception as exc:
        return {"status": f"Bucardo tidak terdeteksi atau gagal dihubungi: {exc}", "sync": ""}


@app.on_event("startup")
def startup() -> None:
    with connect() as conn:
        ensure_postgres_monitor_table(conn)
    postgres_monitor_worker.set_auto_finalize_callback(auto_finalize_callback)
    postgres_monitor_worker.start()


@app.on_event("shutdown")
def shutdown() -> None:
    postgres_monitor_worker.stop()


@app.get("/")
def dashboard(
    request: Request,
    message: str = "",
    notice: str = "",
    _: str = Depends(require_auth),
    conn: sqlite3.Connection = Depends(get_db),
):
    state = load_state()
    current_token = get_setting(conn, "global_token", state.get("token", ""))
    rathole_service = str(state.get("rathole_service_name", "rathole"))
    dashboard_service = str(state.get("dashboard_service_name", "easy-rathole-dashboard"))
    db_sync_mode = state.get("db_sync_mode", {})
    service_ports = normalize_service_ports(state.get("service_ports"), db_sync_mode)
    db_port_mapping = next((row for row in service_ports if str(row.get("name")) == "db"), {})
    db_remote_port = int(db_port_mapping.get("remote_bind_port", 5444))
    db_backend_local_addr = str(db_port_mapping.get("client_local_addr", "127.0.0.1:5444"))
    exposed_ports = exposed_ports_from_service_ports(service_ports, db_sync_mode)
    db_sync_enabled = isinstance(db_sync_mode, dict) and db_sync_mode.get("enabled") is True
    private_sync_tunnel_addr = (
        str(db_sync_mode.get("private_db_tunnel_addr", "127.0.0.1:5444")) if db_sync_enabled else ""
    )
    public_ip = str(state.get("public_ip", "<unknown>"))
    public_db_bind_addr = str(db_sync_mode.get("vps_db_addr", "0.0.0.0:5444")) if db_sync_enabled else ""
    public_db_port = public_db_bind_addr.rsplit(":", 1)[-1] if ":" in public_db_bind_addr else "5444"
    public_db_addr = f"{public_ip}:{public_db_port}" if db_sync_enabled else ""
    control_port = str(state.get("rathole_control_port", "<unknown>"))

    flash_message = message or notice
    flash_type = classify_flash_message(flash_message)
    token_exists = bool(current_token)
    forward_status = build_forward_port_status(exposed_ports)
    postgres_monitor = get_postgres_monitor_latest(conn)

    context = {
        "request": request,
        "flash_message": flash_message,
        "flash_type": flash_type,
        "public_ip": public_ip,
        "control_port": control_port,
        "dashboard_port": state.get("dashboard_port", 8088),
        "token_masked": mask_token(current_token),
        "token_exists": token_exists,
        "rathole_service": rathole_service,
        "rathole_status": service_status(rathole_service),
        "dashboard_service": dashboard_service,
        "dashboard_status": service_status(dashboard_service),
        "exposed_ports": exposed_ports,
        "forward_port_status": forward_status,
        "forward_active_count": sum(1 for row in forward_status if row["status"] == "active"),
        "postgres_monitor": postgres_monitor,
        "db_remote_port": db_remote_port,
        "db_backend_local_addr": db_backend_local_addr,
        "db_sync_enabled": db_sync_enabled,
        "public_db_addr": public_db_addr,
        "public_db_bind_addr": public_db_bind_addr,
        "private_sync_tunnel_addr": private_sync_tunnel_addr,
        "initial_clone_done": bool(db_sync_mode.get("initial_clone_done")) if isinstance(db_sync_mode, dict) else False,
        "bucardo_configured": bool(db_sync_mode.get("bucardo_configured")) if isinstance(db_sync_mode, dict) else False,
        "db_sync_waiting_for_client": bool(db_sync_mode.get("waiting_for_client"))
        if isinstance(db_sync_mode, dict)
        else False,
        "db_sync_status_label": db_sync_status_label(db_sync_mode),
        "db_sync_finalize_available": db_sync_enabled
        and not (isinstance(db_sync_mode, dict) and db_sync_mode.get("bucardo_configured") is True),
        "supported_clients": build_supported_clients(public_ip, control_port),
        "client_tunnel_details": build_client_tunnel_details(service_ports),
        "updated_at": state.get("updated_at", "-"),
    }
    return templates.TemplateResponse("dashboard.html", context)


@app.get("/api/monitor/postgres/latest")
def monitor_postgres_latest(conn: sqlite3.Connection = Depends(get_db)) -> dict[str, object]:
    return get_postgres_monitor_latest(conn)


def mask_token(token: str) -> str:
    if not token:
        return "(not set)"
    if len(token) <= 8:
        return "*" * len(token)
    return f"{token[:4]}{'*' * (len(token) - 8)}{token[-4:]}"


def build_windows_bundle_error_message(
    platform_label: str,
    asset_dir: str,
    required_assets: list[str],
    exc: Exception,
    *,
    includes_gui: bool,
) -> str:
    assets_hint = ", ".join(f"assets/{asset_dir}/{name}" for name in required_assets)
    extra = ""
    if not includes_gui:
        extra = " Bundle ini memang tidak memakai ipos5-rathole-gui.exe."
    return (
        f"Bundle {platform_label} belum siap. {exc}. "
        f"Pastikan aset berikut tersedia pada server resources: {assets_hint}."
        f"{extra}"
    )


@app.post("/db-sync/finalize")
def finalize_db_sync(
    _: str = Depends(require_auth),
):
    state = load_state()
    _ok, msg = run_db_sync_finalize(state)
    return RedirectResponse(
        url=f"/?message={quote_plus(msg)}",
        status_code=status.HTTP_303_SEE_OTHER,
    )


@app.post("/token")
def set_token(
    token: str = Form(...),
    _: str = Depends(require_auth),
    conn: sqlite3.Connection = Depends(get_db),
):
    state = load_state()
    try:
        ok, msg = update_global_token(conn=conn, state=state, token=token)
    except ValueError as exc:
        return RedirectResponse(
            url=f"/?message={quote_plus(str(exc))}",
            status_code=status.HTTP_303_SEE_OTHER,
        )

    if not ok:
        return RedirectResponse(
            url=f"/?message={quote_plus(msg)}",
            status_code=status.HTTP_303_SEE_OTHER,
        )

    return RedirectResponse(
        url=f"/?message={quote_plus(msg)}",
        status_code=status.HTTP_303_SEE_OTHER,
    )


@app.get("/download/windows")
def download_windows(
    _: str = Depends(require_auth),
    conn: sqlite3.Connection = Depends(get_db),
):
    state = load_state()
    token = get_setting(conn, "global_token", state.get("token", ""))
    if not token:
        raise HTTPException(status_code=400, detail="Token belum diset")

    try:
        bundle = generate_windows_bundle(state, token)
    except (FileNotFoundError, RuntimeError) as exc:
        raise HTTPException(
            status_code=400,
            detail=build_windows_bundle_error_message(
                "Windows",
                "windows",
                [
                    WINDOWS_UNIFIED_NAME,
                    WINDOWS_SERVICE_WRAPPER_NAME,
                    WINDOWS_RATHOLE_BINARY_NAME,
                    WINDOWS_GUI_BINARY_NAME,
                    WINDOWS_NSSM_NAME,
                ],
                exc,
                includes_gui=True,
            ),
        ) from exc

    return FileResponse(
        path=bundle,
        media_type="application/zip",
        filename=bundle.name,
    )


@app.get("/download/windows7")
def download_windows7(
    _: str = Depends(require_auth),
    conn: sqlite3.Connection = Depends(get_db),
):
    state = load_state()
    token = get_setting(conn, "global_token", state.get("token", ""))
    if not token:
        raise HTTPException(status_code=400, detail="Token belum diset")

    try:
        bundle = generate_windows7_bundle(state, token)
    except (FileNotFoundError, RuntimeError) as exc:
        raise HTTPException(
            status_code=400,
            detail=build_windows_bundle_error_message(
                "Windows 7",
                "windows7",
                [
                    WINDOWS_UNIFIED_NAME,
                    WINDOWS_SERVICE_WRAPPER_NAME,
                    WINDOWS_RATHOLE_BINARY_NAME,
                    WINDOWS_NSSM_NAME,
                ],
                exc,
                includes_gui=False,
            ),
        ) from exc

    return FileResponse(
        path=bundle,
        media_type="application/zip",
        filename=bundle.name,
    )


@app.get("/download/linux")
def download_linux(
    _: str = Depends(require_auth),
    conn: sqlite3.Connection = Depends(get_db),
):
    state = load_state()
    token = get_setting(conn, "global_token", state.get("token", ""))
    if not token:
        raise HTTPException(status_code=400, detail="Token belum diset")

    bundle = generate_linux_bundle(state, token)
    return FileResponse(
        path=bundle,
        media_type="application/zip",
        filename=bundle.name,
    )
