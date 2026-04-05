from __future__ import annotations

import sqlite3
import subprocess
from pathlib import Path
from urllib.parse import quote_plus

from fastapi import Depends, FastAPI, Form, HTTPException, Request, status
from fastapi.responses import FileResponse, RedirectResponse
from fastapi.security import HTTPBasic, HTTPBasicCredentials
from fastapi.templating import Jinja2Templates

from .auth import verify_password
from .db import connect, get_setting, get_user
from .services.bundle_service import generate_linux_bundle, generate_windows_bundle
from .services.token_service import update_global_token
from .state import load_state

app = FastAPI(title="Easy Rathole Dashboard", version="0.1.0")
security = HTTPBasic()
templates = Jinja2Templates(directory=str(Path(__file__).parent / "templates"))


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


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}


@app.get("/")
def dashboard(
    request: Request,
    message: str = "",
    _: str = Depends(require_auth),
    conn: sqlite3.Connection = Depends(get_db),
):
    state = load_state()
    current_token = get_setting(conn, "global_token", state.get("token", ""))
    rathole_service = str(state.get("rathole_service_name", "rathole"))
    dashboard_service = str(state.get("dashboard_service_name", "easy-rathole-dashboard"))

    context = {
        "request": request,
        "message": message,
        "public_ip": state.get("public_ip", "<unknown>"),
        "control_port": state.get("rathole_control_port", "<unknown>"),
        "dashboard_port": state.get("dashboard_port", 8088),
        "token_masked": mask_token(current_token),
        "token_exists": bool(current_token),
        "rathole_service": rathole_service,
        "rathole_status": service_status(rathole_service),
        "dashboard_service": dashboard_service,
        "dashboard_status": service_status(dashboard_service),
        "exposed_ports": state.get("exposed_ports", [5444, 5480, 5485]),
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
