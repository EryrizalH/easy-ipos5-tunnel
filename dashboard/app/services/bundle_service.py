from __future__ import annotations

import json
import os
import shutil
import tempfile
import zipfile
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from .tunnel_ports import normalize_service_ports

WINDOWS_SERVICE_WRAPPER_NAME = "ipos5-rathole-service.exe"
WINDOWS_RATHOLE_BINARY_NAME = "ipos5-rathole.exe"
WINDOWS_GUI_BINARY_NAME = "ipos5-rathole-gui.exe"
WINDOWS_UNIFIED_NAME = "setup.exe"
WINDOWS_NSSM_NAME = "nssm.exe"
LINUX_SERVICE_NAME = "easy-rathole-client"
WINDOWS_SERVICE_NAME = "EasyRatholeClient"
WINDOWS_ASSET_DIRNAME = "windows"
WINDOWS7_ASSET_DIRNAME = "windows7"


def timestamp_slug() -> str:
    return datetime.now(UTC).strftime("%Y%m%d-%H%M%S")


def get_env_path(name: str, fallback: str) -> Path:
    return Path(os.environ.get(name, fallback))


def bundles_dir() -> Path:
    path = get_env_path("EASY_RATHOLE_BUNDLES_DIR", "/opt/easy-rathole/bundles")
    path.mkdir(parents=True, exist_ok=True)
    return path


def resources_dir() -> Path:
    return get_env_path("EASY_RATHOLE_RESOURCES_DIR", "/opt/easy-rathole/resources")


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
    control_port = str(state.get("rathole_control_port", "2333"))
    service_ports = normalize_service_ports(state.get("service_ports"), state.get("db_sync_mode"))
    by_name = {str(row.get("name", "")).strip(): row for row in service_ports}
    db = by_name.get("db", {})
    pos_http = by_name.get("pos_http", {})
    pos_worker = by_name.get("pos_worker", {})

    return render_template(
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


def generate_windows_bundle(state: dict[str, Any], token: str) -> Path:
    windows_wrapper_bin = windows_asset_path(WINDOWS_ASSET_DIRNAME, WINDOWS_SERVICE_WRAPPER_NAME)
    windows_rathole_bin = windows_asset_path(WINDOWS_ASSET_DIRNAME, WINDOWS_RATHOLE_BINARY_NAME)
    windows_gui_bin = windows_asset_path(WINDOWS_ASSET_DIRNAME, WINDOWS_GUI_BINARY_NAME)
    windows_unified_bin = windows_asset_path(WINDOWS_ASSET_DIRNAME, WINDOWS_UNIFIED_NAME)
    nssm_exe = windows_asset_path(WINDOWS_ASSET_DIRNAME, WINDOWS_NSSM_NAME)
    require_file(windows_wrapper_bin, WINDOWS_SERVICE_WRAPPER_NAME)
    require_file(windows_rathole_bin, WINDOWS_RATHOLE_BINARY_NAME)
    require_file(windows_gui_bin, WINDOWS_GUI_BINARY_NAME)
    require_file(windows_unified_bin, WINDOWS_UNIFIED_NAME)
    require_file(nssm_exe, WINDOWS_NSSM_NAME)

    bundle_name = f"windows-client-{timestamp_slug()}.zip"
    out_path = bundles_dir() / bundle_name

    temp_dir = Path(tempfile.mkdtemp(prefix="easy-rathole-win-"))
    try:
        shutil.copy2(windows_wrapper_bin, temp_dir / WINDOWS_SERVICE_WRAPPER_NAME)
        shutil.copy2(windows_rathole_bin, temp_dir / WINDOWS_RATHOLE_BINARY_NAME)
        shutil.copy2(windows_gui_bin, temp_dir / WINDOWS_GUI_BINARY_NAME)
        shutil.copy2(windows_unified_bin, temp_dir / WINDOWS_UNIFIED_NAME)
        shutil.copy2(nssm_exe, temp_dir / WINDOWS_NSSM_NAME)

        (temp_dir / "client.toml").write_text(render_client_toml(state, token), encoding="utf-8")

        (temp_dir / "README.txt").write_text(
            "\n".join(
                [
                    "IPOS5TunnelPublik - Client Windows",
                    "",
                    "1) Ekstrak file ZIP ini.",
                    "2) Jalankan setup.exe sebagai Administrator.",
                    "3) Gunakan menu aplikasi untuk:",
                    "   - Install IP Public",
                    "   - Uninstall Service IP Public",
                    "   - Kunci/Lepas Kunci pembuatan database baru",
                    "4) Arsitektur DB:",
                    "   - Tanpa DB sync: rathole DB local_addr default 127.0.0.1:5444",
                    "   - Dengan DB sync: rathole DB local_addr default 127.0.0.1:5444",
                    "5) Saat Install Service, aplikasi otomatis membuat shortcut desktop",
                    "   'ipos5-rathole' untuk membuka GUI jendela utama dengan Run as Administrator (UAC prompt).",
                    "6) GUI tidak autostart saat login Windows; buka manual via shortcut desktop.",
                    "7) Saat Uninstall Service, shortcut desktop GUI ikut dihapus.",
                    f"8) Service default yang dipakai: {WINDOWS_SERVICE_NAME}",
                    "9) setup.exe adalah installer interaktif; service Windows tidak menjalankan setup.exe.",
                    f"10) Service wrapper {WINDOWS_SERVICE_WRAPPER_NAME} akan menjalankan {WINDOWS_RATHOLE_BINARY_NAME} dengan client.toml.",
                    "11) Script template lama (setup-client.cmd/install-service.cmd) bukan jalur utama bundle dashboard.",
                    "12) Paket ini wajib utuh:",
                    "   setup.exe + ipos5-rathole-service.exe + rathole.exe + ipos5-rathole-gui.exe + nssm.exe + client.toml",
                ]
            )
            + "\n",
            encoding="utf-8",
        )

        with zipfile.ZipFile(out_path, "w", compression=zipfile.ZIP_DEFLATED) as zf:
            for child in temp_dir.iterdir():
                zf.write(child, arcname=child.name)
    finally:
        shutil.rmtree(temp_dir, ignore_errors=True)

    return out_path


def generate_windows7_bundle(state: dict[str, Any], token: str) -> Path:
    windows_wrapper_bin = windows_asset_path(WINDOWS7_ASSET_DIRNAME, WINDOWS_SERVICE_WRAPPER_NAME)
    windows_rathole_bin = windows_asset_path(WINDOWS7_ASSET_DIRNAME, WINDOWS_RATHOLE_BINARY_NAME)
    windows_unified_bin = windows_asset_path(WINDOWS7_ASSET_DIRNAME, WINDOWS_UNIFIED_NAME)
    nssm_exe = windows_asset_path(WINDOWS7_ASSET_DIRNAME, WINDOWS_NSSM_NAME)
    require_file(windows_wrapper_bin, WINDOWS_SERVICE_WRAPPER_NAME)
    require_file(windows_rathole_bin, WINDOWS_RATHOLE_BINARY_NAME)
    require_file(windows_unified_bin, WINDOWS_UNIFIED_NAME)
    require_file(nssm_exe, WINDOWS_NSSM_NAME)

    bundle_name = f"windows7-client-{timestamp_slug()}.zip"
    out_path = bundles_dir() / bundle_name

    temp_dir = Path(tempfile.mkdtemp(prefix="easy-rathole-win7-"))
    try:
        shutil.copy2(windows_wrapper_bin, temp_dir / WINDOWS_SERVICE_WRAPPER_NAME)
        shutil.copy2(windows_rathole_bin, temp_dir / WINDOWS_RATHOLE_BINARY_NAME)
        shutil.copy2(windows_unified_bin, temp_dir / WINDOWS_UNIFIED_NAME)
        shutil.copy2(nssm_exe, temp_dir / WINDOWS_NSSM_NAME)

        (temp_dir / "client.toml").write_text(render_client_toml(state, token), encoding="utf-8")

        (temp_dir / "README.txt").write_text(
            "\n".join(
                [
                    "IPOS5TunnelPublik - Client Windows 7",
                    "",
                    "1) Ekstrak file ZIP ini.",
                    "2) Jalankan setup.exe sebagai Administrator.",
                    "3) Paket Windows 7 ini fokus ke service tunnel dan kompatibilitas Win7.",
                    "4) Bundle ini tidak menyertakan GUI desktop (ipos5-rathole-gui.exe).",
                    "5) Installer/service Win7 harus berasal dari aset kompatibel di assets/windows7.",
                    "6) Arsitektur DB:",
                    "   - Tanpa DB sync: rathole DB local_addr default 127.0.0.1:5444",
                    "   - Dengan DB sync: rathole DB local_addr default 127.0.0.1:5444",
                    "7) Paket ini wajib utuh:",
                    "   setup.exe + ipos5-rathole-service.exe + ipos5-rathole.exe + nssm.exe + client.toml",
                ]
            )
            + "\n",
            encoding="utf-8",
        )

        with zipfile.ZipFile(out_path, "w", compression=zipfile.ZIP_DEFLATED) as zf:
            for child in temp_dir.iterdir():
                zf.write(child, arcname=child.name)
    finally:
        shutil.rmtree(temp_dir, ignore_errors=True)

    return out_path


def generate_linux_bundle(state: dict[str, Any], token: str) -> Path:
    bundle_name = f"linux-client-{timestamp_slug()}.zip"
    out_path = bundles_dir() / bundle_name

    temp_dir = Path(tempfile.mkdtemp(prefix="easy-rathole-linux-"))
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

        (temp_dir / "README.txt").write_text(
            "\n".join(
                [
                    "IPOS5TunnelPublik - Client Linux",
                    "",
                    "1) Ekstrak paket ini di mesin client Linux.",
                    "2) Jalankan: sudo ./install-client.sh",
                    "3) Service client akan aktif otomatis saat boot.",
                ]
            )
            + "\n",
            encoding="utf-8",
        )

        with zipfile.ZipFile(out_path, "w", compression=zipfile.ZIP_DEFLATED) as zf:
            for child in temp_dir.iterdir():
                zf.write(child, arcname=child.name)
    finally:
        shutil.rmtree(temp_dir, ignore_errors=True)

    return out_path
