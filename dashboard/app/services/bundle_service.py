from __future__ import annotations

import json
import os
import shutil
import tempfile
import zipfile
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from .tunnel_ports import DEFAULT_RATHOLE_CONTROL_PORT, normalize_service_ports

WINDOWS_SERVICE_WRAPPER_NAME = "nusatunnel-service.exe"
WINDOWS_RATHOLE_BINARY_NAME = "nusatunnel.exe"
WINDOWS_GUI_BINARY_NAME = "nusatunnel-gui.exe"
WINDOWS_UNIFIED_NAME = "setup.exe"
WINDOWS_NSSM_NAME = "nssm.exe"
WINDOWS_PGBOUNCER_BINARY_NAME = "pgbouncer.exe"
WINDOWS_PGBOUNCER_LIBEVENT_NAME = "libevent-7.dll"
WINDOWS_PGBOUNCER_LIBSSL_NAME = "libssl-3-x64.dll"
WINDOWS_PGBOUNCER_LIBCRYPTO_NAME = "libcrypto-3-x64.dll"
WINDOWS_PGBOUNCER_LIBWINPTH_NAME = "libwinpthread-1.dll"
WINDOWS_PGBOUNCER_INI_NAME = "pgbouncer.ini"
WINDOWS_PGBOUNCER_DATABASES_NAME = "pgbouncer-databases.json"
WINDOWS_PGBOUNCER_USERLIST_NAME = "userlist.sample.txt"
LINUX_SERVICE_NAME = "nusatunnel-client"
WINDOWS_SERVICE_NAME = "NusaTunnelClient"
WINDOWS_ASSET_DIRNAME = "windows"
WINDOWS7_ASSET_DIRNAME = "windows7"
PGBOUNCER_METADATA_PREFIX = "# nusatunnel-pgbouncer-databases: "

WINDOWS_EMBEDDED_RUNTIME_NAMES = (
    WINDOWS_NSSM_NAME,
    WINDOWS_SERVICE_WRAPPER_NAME,
    WINDOWS_RATHOLE_BINARY_NAME,
    WINDOWS_PGBOUNCER_BINARY_NAME,
    WINDOWS_PGBOUNCER_LIBEVENT_NAME,
    WINDOWS_PGBOUNCER_LIBSSL_NAME,
    WINDOWS_PGBOUNCER_LIBCRYPTO_NAME,
    WINDOWS_PGBOUNCER_LIBWINPTH_NAME,
)


def timestamp_slug() -> str:
    return datetime.now(UTC).strftime("%Y%m%d-%H%M%S")


def get_env_path(name: str, fallback: str) -> Path:
    return Path(os.environ.get(name, fallback))


def bundles_dir() -> Path:
    path = get_env_path("NUSA_TUNNEL_BUNDLES_DIR", "/opt/nusatunnel/bundles")
    path.mkdir(parents=True, exist_ok=True)
    return path


def resources_dir() -> Path:
    return get_env_path("NUSA_TUNNEL_RESOURCES_DIR", "/opt/nusatunnel/resources")


def windows_asset_path(asset_dir: str, name: str) -> Path:
    return resources_dir() / f"assets/{asset_dir}/{name}"


def require_file(path: Path, label: str) -> None:
    if not path.exists():
        raise FileNotFoundError(f"{label} not found: {path}")
    if path.stat().st_size <= 0:
        raise RuntimeError(f"{label} invalid (empty file): {path}")


def render_template(template_path: Path, replacements: dict[str, str]) -> str:
    text = template_path.read_text(encoding="utf-8")
    for key, value in replacements.items():
        text = text.replace("{{" + key + "}}", value)
    return text


def render_client_toml(state: dict[str, Any], token: str) -> str:
    template_path = resources_dir() / "templates/rathole/client.toml.tpl"
    if not template_path.exists():
        raise FileNotFoundError(f"Client template not found: {template_path}")

    server_addr = str(state.get("public_ip", "127.0.0.1"))
    control_port = str(state.get("rathole_control_port", DEFAULT_RATHOLE_CONTROL_PORT))
    service_ports = normalize_service_ports(state.get("service_ports"))
    by_name = {str(row.get("name", "")).strip(): row for row in service_ports}
    db = by_name.get("db", {})
    pos_http = by_name.get("pos_http", {})
    pos_worker = by_name.get("pos_worker", {})

    client_toml = render_template(
        template_path,
        {
            "SERVER_ADDR": server_addr,
            "RATHOLE_CONTROL_PORT": control_port,
            "GLOBAL_TOKEN": token,
            "DB_SERVICE_KEY": str(db.get("service_key", "port_5444")),
            "DB_CLIENT_LOCAL_ADDR": str(db.get("client_local_addr", "127.0.0.1:5444")),
            "POS_HTTP_SERVICE_KEY": str(pos_http.get("service_key", "port_5480")),
            "POS_HTTP_CLIENT_LOCAL_ADDR": str(pos_http.get("client_local_addr", "127.0.0.1:5480")),
            "POS_WORKER_SERVICE_KEY": str(pos_worker.get("service_key", "port_5485")),
            "POS_WORKER_CLIENT_LOCAL_ADDR": str(pos_worker.get("client_local_addr", "127.0.0.1:5485")),
        },
    )
    metadata = json.dumps(
        {"databases": normalize_pgbouncer_databases(state.get("pgbouncer_databases"))},
        separators=(",", ":"),
        sort_keys=True,
    )
    return f"{client_toml.rstrip()}\n\n{PGBOUNCER_METADATA_PREFIX}{metadata}\n"


def normalize_pgbouncer_databases(raw: Any) -> list[dict[str, str]]:
    default_entry = [{"name": "postgres", "backend_dbname": "postgres"}]
    if raw is None:
        return default_entry
    if not isinstance(raw, list):
        return default_entry

    normalized: list[dict[str, str]] = []
    seen: set[str] = set()
    for item in raw:
        if isinstance(item, str):
            name = item.strip()
            backend_dbname = name
        elif isinstance(item, dict):
            name = str(item.get("name", "")).strip()
            backend_dbname = str(item.get("backend_dbname", "")).strip() or name
        else:
            continue

        if not name:
            continue

        dedupe_key = name.lower()
        if dedupe_key in seen:
            continue
        seen.add(dedupe_key)
        normalized.append({"name": name, "backend_dbname": backend_dbname})

    return normalized or default_entry


def render_pgbouncer_databases_json(state: dict[str, Any]) -> str:
    payload = {"databases": normalize_pgbouncer_databases(state.get("pgbouncer_databases"))}
    return json.dumps(payload, indent=2, sort_keys=True) + "\n"


def require_windows_installer_payload(installer: Path, *, includes_gui: bool) -> None:
    require_file(installer, WINDOWS_UNIFIED_NAME)
    try:
        with zipfile.ZipFile(installer) as payload:
            names = set(payload.namelist())
    except zipfile.BadZipFile as exc:
        raise RuntimeError(f"{WINDOWS_UNIFIED_NAME} bukan installer mandiri yang valid") from exc

    missing = [name for name in WINDOWS_EMBEDDED_RUNTIME_NAMES if name not in names]
    if missing:
        raise RuntimeError(f"payload {WINDOWS_UNIFIED_NAME} tidak lengkap: {', '.join(missing)}")
    if includes_gui and WINDOWS_GUI_BINARY_NAME not in names:
        raise RuntimeError(f"payload {WINDOWS_UNIFIED_NAME} tidak lengkap: {WINDOWS_GUI_BINARY_NAME}")
    if not includes_gui and WINDOWS_GUI_BINARY_NAME in names:
        raise RuntimeError(f"payload {WINDOWS_UNIFIED_NAME} Windows 7 tidak boleh memuat {WINDOWS_GUI_BINARY_NAME}")


def generate_windows_bundle(state: dict[str, Any], token: str) -> Path:
    windows_unified_bin = windows_asset_path(WINDOWS_ASSET_DIRNAME, WINDOWS_UNIFIED_NAME)
    require_windows_installer_payload(windows_unified_bin, includes_gui=True)

    bundle_name = f"windows-client-{timestamp_slug()}.zip"
    out_path = bundles_dir() / bundle_name

    temp_dir = Path(tempfile.mkdtemp(prefix="nusatunnel-win-"))
    try:
        shutil.copy2(windows_unified_bin, temp_dir / WINDOWS_UNIFIED_NAME)
        (temp_dir / "client.toml").write_text(render_client_toml(state, token), encoding="utf-8")

        with zipfile.ZipFile(out_path, "w", compression=zipfile.ZIP_DEFLATED) as zf:
            for child in temp_dir.iterdir():
                zf.write(child, arcname=child.name)
    finally:
        shutil.rmtree(temp_dir, ignore_errors=True)

    return out_path


def generate_windows7_bundle(state: dict[str, Any], token: str) -> Path:
    windows_unified_bin = windows_asset_path(WINDOWS7_ASSET_DIRNAME, WINDOWS_UNIFIED_NAME)
    require_windows_installer_payload(windows_unified_bin, includes_gui=False)

    bundle_name = f"windows7-client-{timestamp_slug()}.zip"
    out_path = bundles_dir() / bundle_name

    temp_dir = Path(tempfile.mkdtemp(prefix="nusatunnel-win7-"))
    try:
        shutil.copy2(windows_unified_bin, temp_dir / WINDOWS_UNIFIED_NAME)
        (temp_dir / "client.toml").write_text(render_client_toml(state, token), encoding="utf-8")

        with zipfile.ZipFile(out_path, "w", compression=zipfile.ZIP_DEFLATED) as zf:
            for child in temp_dir.iterdir():
                zf.write(child, arcname=child.name)
    finally:
        shutil.rmtree(temp_dir, ignore_errors=True)

    return out_path


def generate_linux_bundle(state: dict[str, Any], token: str) -> Path:
    bundle_name = f"linux-client-{timestamp_slug()}.zip"
    out_path = bundles_dir() / bundle_name

    temp_dir = Path(tempfile.mkdtemp(prefix="nusatunnel-linux-"))
    try:
        (temp_dir / "client.toml").write_text(render_client_toml(state, token), encoding="utf-8")

        install_tpl = resources_dir() / "assets/linux/install-client.sh.tpl"
        install_script = render_template(
            install_tpl,
            {
                "LINUX_SERVICE_NAME": LINUX_SERVICE_NAME,
            },
        )
        install_path = temp_dir / "install-client.sh"
        install_path.write_text(install_script, encoding="utf-8")
        install_path.chmod(0o755)

        with zipfile.ZipFile(out_path, "w", compression=zipfile.ZIP_DEFLATED) as zf:
            for child in temp_dir.iterdir():
                zf.write(child, arcname=child.name)
    finally:
        shutil.rmtree(temp_dir, ignore_errors=True)

    return out_path
