from __future__ import annotations

import os
import shutil
import tempfile
import zipfile
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

WINDOWS_BINARY_NAME = "ipos5-rathole.exe"
WINDOWS_GUI_BINARY_NAME = "ipos5-rathole-gui.exe"
WINDOWS_UNIFIED_NAME = "setup.exe"
WINDOWS_NSSM_NAME = "nssm.exe"
LINUX_SERVICE_NAME = "easy-rathole-client"
WINDOWS_SERVICE_NAME = "EasyRatholeClient"


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
    return render_template(
        template_path,
        {
            "SERVER_ADDR": server_addr,
            "RATHOLE_CONTROL_PORT": control_port,
            "GLOBAL_TOKEN": token,
        },
    )


def generate_windows_bundle(state: dict[str, Any], token: str) -> Path:
    windows_bin = resources_dir() / f"assets/windows/{WINDOWS_BINARY_NAME}"
    windows_gui_bin = resources_dir() / f"assets/windows/{WINDOWS_GUI_BINARY_NAME}"
    windows_unified_bin = resources_dir() / f"assets/windows/{WINDOWS_UNIFIED_NAME}"
    nssm_exe = resources_dir() / f"assets/windows/{WINDOWS_NSSM_NAME}"
    require_file(windows_bin, WINDOWS_BINARY_NAME)
    require_file(windows_gui_bin, WINDOWS_GUI_BINARY_NAME)
    require_file(windows_unified_bin, WINDOWS_UNIFIED_NAME)
    require_file(nssm_exe, WINDOWS_NSSM_NAME)

    bundle_name = f"windows-client-{timestamp_slug()}.zip"
    out_path = bundles_dir() / bundle_name

    temp_dir = Path(tempfile.mkdtemp(prefix="easy-rathole-win-"))
    try:
        shutil.copy2(windows_bin, temp_dir / WINDOWS_BINARY_NAME)
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
                    "   - Install Service IP Public",
                    "   - Uninstall Service IP Public",
                    "   - Kunci/Lepas Kunci pembuatan database baru",
                    "4) Saat Install Service, aplikasi otomatis membuat shortcut desktop",
                    "   'ipos5-rathole' untuk membuka GUI dengan Run as Administrator (UAC prompt).",
                    "5) Saat Uninstall Service, shortcut desktop GUI ikut dihapus.",
                    f"6) Service default yang dipakai: {WINDOWS_SERVICE_NAME}",
                    "7) Paket ini wajib utuh:",
                    "   setup.exe + ipos5-rathole.exe + ipos5-rathole-gui.exe + nssm.exe + client.toml",
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
