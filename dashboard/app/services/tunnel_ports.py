from __future__ import annotations

from copy import deepcopy
from typing import Any

DEFAULT_SERVICE_PORTS: list[dict[str, Any]] = [
    {
        "name": "db",
        "service_key": "port_5444",
        "protocol": "tcp",
        "remote_bind_port": 5444,
        "client_local_addr": "127.0.0.1:5444",
        "client_local_port": 5444,
    },
    {
        "name": "pos_http",
        "service_key": "port_5480",
        "protocol": "tcp",
        "remote_bind_port": 5480,
        "client_local_addr": "127.0.0.1:5480",
        "client_local_port": 5480,
    },
    {
        "name": "pos_worker",
        "service_key": "port_5485",
        "protocol": "tcp",
        "remote_bind_port": 5485,
        "client_local_addr": "127.0.0.1:5485",
        "client_local_port": 5485,
    },
]


def default_service_ports() -> list[dict[str, Any]]:
    return deepcopy(DEFAULT_SERVICE_PORTS)


def normalize_service_ports(raw: Any) -> list[dict[str, Any]]:
    defaults = default_service_ports()
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
        for key in ("service_key", "protocol", "client_local_addr"):
            value = provided.get(key)
            if isinstance(value, str) and value.strip():
                row[key] = value.strip()

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
                "remote_bind_port": remote_bind_port,
                "client_local_addr": client_local_addr,
                "client_local_port": client_local_port,
            }
        )

    return merged


def exposed_ports_from_service_ports(service_ports: list[dict[str, Any]]) -> list[int]:
    exposed_ports: list[int] = []
    for row in service_ports:
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
