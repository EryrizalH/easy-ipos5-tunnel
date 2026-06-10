from __future__ import annotations

from copy import deepcopy
from typing import Any

DEFAULT_SERVICE_PORTS: list[dict[str, Any]] = [
    {
        "name": "db",
        "service_key": "port_5444",
        "protocol": "tcp",
        "bind_addr": "0.0.0.0",
        "expose_public": True,
        "remote_bind_port": 5444,
        "client_local_addr": "127.0.0.1:5444",
        "client_local_port": 5444,
    },
    {
        "name": "pos_http",
        "service_key": "port_5480",
        "protocol": "tcp",
        "bind_addr": "0.0.0.0",
        "expose_public": True,
        "remote_bind_port": 5480,
        "client_local_addr": "127.0.0.1:5480",
        "client_local_port": 5480,
    },
    {
        "name": "pos_worker",
        "service_key": "port_5485",
        "protocol": "tcp",
        "bind_addr": "0.0.0.0",
        "expose_public": True,
        "remote_bind_port": 5485,
        "client_local_addr": "127.0.0.1:5485",
        "client_local_port": 5485,
    },
]


def default_service_ports() -> list[dict[str, Any]]:
    return deepcopy(DEFAULT_SERVICE_PORTS)


def db_sync_enabled(raw: Any) -> bool:
    return isinstance(raw, dict) and raw.get("enabled") is True


def private_db_backend_mode(raw: Any) -> str:
    if not isinstance(raw, dict):
        return "direct"
    value = str(raw.get("private_db_backend_mode", "direct")).strip().lower()
    if value == "pgbouncer_backend":
        return value
    return "direct"


def apply_db_sync_mode(defaults: list[dict[str, Any]], db_sync_mode: Any) -> list[dict[str, Any]]:
    if not db_sync_enabled(db_sync_mode):
        return defaults

    backend_mode = private_db_backend_mode(db_sync_mode)
    private_local_addr = "127.0.0.1:5445" if backend_mode == "pgbouncer_backend" else "127.0.0.1:5444"

    out = deepcopy(defaults)
    for row in out:
        if row.get("name") != "db":
            continue
        row.update(
            {
                "service_key": "port_5445",
                "bind_addr": "127.0.0.1",
                "expose_public": False,
                "remote_bind_port": 5445,
                "client_local_addr": private_local_addr,
                "client_local_port": int(private_local_addr.rsplit(":", 1)[-1]),
            }
        )
        if isinstance(db_sync_mode, dict):
            tunnel_addr = str(db_sync_mode.get("private_db_tunnel_addr", "")).strip()
            if ":" in tunnel_addr:
                host, port = tunnel_addr.rsplit(":", 1)
                if host.strip() and port.isdigit():
                    row["bind_addr"] = host.strip()
                    row["remote_bind_port"] = int(port)
            vps_addr = str(db_sync_mode.get("vps_db_addr", "")).strip()
            if vps_addr:
                row["vps_db_addr"] = vps_addr
        break
    return out


def normalize_service_ports(raw: Any, db_sync_mode: Any = None) -> list[dict[str, Any]]:
    defaults = apply_db_sync_mode(default_service_ports(), db_sync_mode)
    sync_enabled = db_sync_enabled(db_sync_mode)
    if not isinstance(raw, list):
        return defaults

    merged: list[dict[str, Any]] = []
    raw_by_name: dict[str, dict[str, Any]] = {}
    for row in raw:
        if not isinstance(row, dict):
            continue
        name = str(row.get("name", "")).strip()
        if name:
            raw_by_name[name] = row

    for default in defaults:
        row = default.copy()
        provided = raw_by_name.get(default["name"], {})
        if sync_enabled and default["name"] == "db":
            merged.append(row)
            continue
        for key in ("service_key", "protocol", "bind_addr", "client_local_addr"):
            value = provided.get(key)
            if isinstance(value, str) and value.strip():
                row[key] = value.strip()

        value = provided.get("expose_public")
        if isinstance(value, bool):
            row["expose_public"] = value

        for key in ("remote_bind_port", "client_local_port"):
            value = provided.get(key)
            try:
                if value is not None:
                    row[key] = int(value)
            except (TypeError, ValueError):
                pass

        merged.append(row)

    default_names = {row["name"] for row in defaults}
    for row in raw:
        if not isinstance(row, dict):
            continue
        name = str(row.get("name", "")).strip()
        if not name or name in default_names:
            continue

        service_key = str(row.get("service_key", "")).strip()
        protocol = str(row.get("protocol", "tcp")).strip().lower() or "tcp"
        bind_addr = str(row.get("bind_addr", "0.0.0.0")).strip() or "0.0.0.0"
        expose_public = row.get("expose_public")
        if not isinstance(expose_public, bool):
            expose_public = bind_addr not in {"127.0.0.1", "localhost", "::1"}
        client_local_addr = str(row.get("client_local_addr", "")).strip()
        raw_remote = row.get("remote_bind_port")
        raw_local = row.get("client_local_port")
        if raw_remote is None or raw_local is None:
            continue
        try:
            remote_bind_port = int(raw_remote)
            client_local_port = int(raw_local)
        except (TypeError, ValueError):
            continue
        if not service_key or not client_local_addr:
            continue

        merged.append(
            {
                "name": name,
                "service_key": service_key,
                "protocol": protocol,
                "bind_addr": bind_addr,
                "expose_public": expose_public,
                "remote_bind_port": remote_bind_port,
                "client_local_addr": client_local_addr,
                "client_local_port": client_local_port,
            }
        )

    return merged


def _port_from_addr(addr: Any, fallback: int) -> int:
    text = str(addr or "").strip()
    if ":" not in text:
        return fallback
    raw_port = text.rsplit(":", 1)[-1]
    if raw_port.isdigit():
        return int(raw_port)
    return fallback


def exposed_ports_from_service_ports(
    service_ports: list[dict[str, Any]], db_sync_mode: Any = None
) -> list[int]:
    exposed_ports: list[int] = []
    if db_sync_enabled(db_sync_mode):
        exposed_ports.append(_port_from_addr(db_sync_mode.get("vps_db_addr"), 5444))

    for row in service_ports:
        if row.get("expose_public") is False:
            continue
        raw_port = row.get("remote_bind_port")
        if raw_port is None:
            continue
        try:
            port = int(raw_port)
        except (TypeError, ValueError):
            continue
        if port not in exposed_ports:
            exposed_ports.append(port)
    return exposed_ports
