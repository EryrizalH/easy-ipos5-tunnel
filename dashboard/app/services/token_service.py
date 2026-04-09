from __future__ import annotations

import subprocess
from pathlib import Path
from typing import Any

from ..db import set_setting
from ..state import merge_state
from .tunnel_ports import exposed_ports_from_service_ports, normalize_service_ports


def validate_token(token: str) -> str:
    token = token.strip()
    if len(token) < 8:
        raise ValueError("Token minimal 8 karakter.")
    if len(token) > 128:
        raise ValueError("Token maksimal 128 karakter.")
    return token


def build_server_config(control_port: int, token: str, service_ports: list[dict[str, Any]]) -> str:
    parts = [
        "[server]",
        f'bind_addr = "0.0.0.0:{control_port}"',
        "",
    ]

    for row in service_ports:
        service_key = str(row.get("service_key", "")).strip()
        if not service_key:
            continue
        protocol = str(row.get("protocol", "tcp")).strip().lower() or "tcp"
        raw_port = row.get("remote_bind_port")
        if raw_port is None:
            continue
        try:
            remote_port = int(raw_port)
        except (TypeError, ValueError):
            continue

        parts.extend(
            [
                f"[server.services.{service_key}]",
                f'type = "{protocol}"',
                f'token = "{token}"',
                f'bind_addr = "0.0.0.0:{remote_port}"',
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
    service_ports = normalize_service_ports(state.get("service_ports"))

    config_text = build_server_config(control_port=control_port, token=token, service_ports=service_ports)
    config_path.parent.mkdir(parents=True, exist_ok=True)
    config_path.write_text(config_text, encoding="utf-8")

    set_setting(conn, "global_token", token)
    conn.commit()

    success, detail = restart_service(service_name)
    merge_state(
        {
            "token": token,
            "service_ports": service_ports,
            "exposed_ports": exposed_ports_from_service_ports(service_ports),
        }
    )

    if success:
        return True, "Token berhasil diperbarui dan service rathole direstart."
    return False, f"Token tersimpan, tapi restart service gagal: {detail}"
