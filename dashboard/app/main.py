from __future__ import annotations

import sqlite3
import socket
import subprocess
from pathlib import Path
from typing import Any
from urllib.parse import quote_plus

from fastapi import Depends, FastAPI, Form, HTTPException, Request, status
from fastapi.responses import FileResponse, RedirectResponse
from fastapi.security import HTTPBasic, HTTPBasicCredentials
from fastapi.templating import Jinja2Templates

from .auth import verify_password
from .db import connect, get_setting, get_user
from .services.bundle_service import (
    LINUX_SERVICE_NAME,
    WINDOWS_BINARY_NAME,
    WINDOWS_GUI_BINARY_NAME,
    WINDOWS_NSSM_NAME,
    WINDOWS_SERVICE_NAME,
    generate_linux_bundle,
    generate_windows_bundle,
)
from .services.token_service import update_global_token
from .state import load_state

app = FastAPI(title="IPOS5TunnelPublik Dashboard", version="0.2.0")
security = HTTPBasic()
templates = Jinja2Templates(directory=str(Path(__file__).parent / "templates"))


def classify_flash_message(message: str) -> str:
    if not message:
        return "info"

    lowered = message.lower()
    if any(word in lowered for word in ("berhasil", "sukses", "selesai", "ok")):
        return "success"
    if any(word in lowered for word in ("gagal", "error", "tidak", "invalid", "failed")):
        return "error"
    if "tapi" in lowered:
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
            "delivery": "Paket ZIP (ipos5-rathole.exe + ipos5-rathole-gui.exe + nssm.exe + client.toml + script setup)",
            "binary_source": (
                "Bundled dari aset lokal dashboard: "
                f"{WINDOWS_BINARY_NAME} + {WINDOWS_GUI_BINARY_NAME} + {WINDOWS_NSSM_NAME}"
            ),
            "setup_hint": "setup-client.cmd (auto UAC/Admin)",
            "remote_endpoint": endpoint,
        },
    ]


def build_client_tunnel_details(exposed_ports: list[Any]) -> list[dict[str, str]]:
    rows: list[dict[str, str]] = []
    for item in exposed_ports:
        try:
            port = int(item)
        except (TypeError, ValueError):
            continue

        rows.append(
            {
                "service": f"port_{port}",
                "protocol": "tcp",
                "remote_port": str(port),
                "client_local_addr": f"127.0.0.1:{port}",
            }
        )
    return rows


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}


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
    raw_exposed_ports = state.get("exposed_ports", [5444, 5480, 5485])
    if isinstance(raw_exposed_ports, list):
        exposed_ports = raw_exposed_ports
    else:
        exposed_ports = [raw_exposed_ports]
    public_ip = str(state.get("public_ip", "<unknown>"))
    control_port = str(state.get("rathole_control_port", "<unknown>"))

    flash_message = message or notice
    flash_type = classify_flash_message(flash_message)
    token_exists = bool(current_token)
    forward_status = build_forward_port_status(exposed_ports)

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
        "supported_clients": build_supported_clients(public_ip, control_port),
        "client_tunnel_details": build_client_tunnel_details(exposed_ports),
        "updated_at": state.get("updated_at", "-"),
    }
    return templates.TemplateResponse("dashboard.html", context)


def mask_token(token: str) -> str:
    if not token:
        return "(not set)"
    if len(token) <= 8:
        return "*" * len(token)
    return f"{token[:4]}{'*' * (len(token) - 8)}{token[-4:]}"


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

    bundle = generate_windows_bundle(state, token)
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
