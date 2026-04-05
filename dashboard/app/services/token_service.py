from __future__ import annotations

import subprocess
from pathlib import Path
from typing import Any

from ..db import set_setting
from ..state import merge_state

FIXED_PORTS = (5444, 5480, 5485)


def validate_token(token: str) -> str:
    token = token.strip()
    if len(token) < 8:
        raise ValueError("Token minimal 8 karakter.")
    if len(token) > 128:
        raise ValueError("Token maksimal 128 karakter.")
    return token


def build_server_config(control_port: int, token: str) -> str:
    parts = [
        "[server]",
        f'bind_addr = "0.0.0.0:{control_port}"',
        "",
    ]

    for port in FIXED_PORTS:
        parts.extend(
            [
                f"[server.services.port_{port}]",
                'type = "tcp"',
                f'token = "{token}"',
                f'bind_addr = "0.0.0.0:{port}"',
                "",
            ]
        )

    return "\n".join(parts).strip() + "\n"


def restart_service(service_name: str) -> tuple[bool, str]:
    try:
        completed = subprocess.run(
            ["systemctl", "restart", service_name],
            check=False,
            capture_output=True,
            text=True,
        )
    except FileNotFoundError:
        return False, "systemctl tidak tersedia"

    if completed.returncode == 0:
        return True, "ok"

    error = (completed.stderr or completed.stdout or "unknown error").strip()
    return False, error


def update_global_token(
    conn,
    state: dict[str, Any],
    token: str,
) -> tuple[bool, str]:
    token = validate_token(token)

    config_path = Path(state.get("rathole_config_path", "/etc/easy-rathole/server.toml"))
    control_port = int(state.get("rathole_control_port", 2333))
    service_name = str(state.get("rathole_service_name", "rathole"))

    config_text = build_server_config(control_port=control_port, token=token)
    config_path.parent.mkdir(parents=True, exist_ok=True)
    config_path.write_text(config_text, encoding="utf-8")

    set_setting(conn, "global_token", token)
    conn.commit()

    success, detail = restart_service(service_name)
    merge_state({"token": token})

    if success:
        return True, "Token berhasil diperbarui dan service rathole direstart."
    return False, f"Token tersimpan, tapi restart service gagal: {detail}"
