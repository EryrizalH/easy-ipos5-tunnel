from __future__ import annotations

import io
import json
import os
import shutil
import tempfile
import urllib.request
import zipfile
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

WINDOWS_ASSET_NAME = "rathole-x86_64-pc-windows-msvc.zip"
LINUX_SERVICE_NAME = "easy-rathole-client"
WINDOWS_SERVICE_NAME = "EasyRatholeClient"
RELEASE_API = "https://api.github.com/repos/rathole-org/rathole/releases/latest"


def timestamp_slug() -> str:
    return datetime.now(UTC).strftime("%Y%m%d-%H%M%S")


def get_env_path(name: str, fallback: str) -> Path:
    return Path(os.environ.get(name, fallback))


def bundles_dir() -> Path:
    path = get_env_path("EASY_RATHOLE_BUNDLES_DIR", "/opt/easy-rathole/bundles")
    path.mkdir(parents=True, exist_ok=True)
    return path


def cache_dir() -> Path:
    path = get_env_path("EASY_RATHOLE_CACHE_DIR", "/opt/easy-rathole/cache")
    path.mkdir(parents=True, exist_ok=True)
    return path


def resources_dir() -> Path:
    return get_env_path("EASY_RATHOLE_RESOURCES_DIR", "/opt/easy-rathole/resources")


def fetch_latest_release() -> dict[str, Any]:
    req = urllib.request.Request(RELEASE_API, headers={"User-Agent": "easy-rathole"})
    with urllib.request.urlopen(req, timeout=20) as resp:
        return json.loads(resp.read().decode("utf-8"))


def get_asset_url(release: dict[str, Any], asset_name: str) -> str:
    for asset in release.get("assets", []):
        if asset.get("name") == asset_name:
            return str(asset.get("browser_download_url", ""))
    return ""


def cached_download(url: str, cache_file: Path) -> Path:
    if cache_file.exists() and cache_file.stat().st_size > 0:
        return cache_file

    req = urllib.request.Request(url, headers={"User-Agent": "easy-rathole"})
    with urllib.request.urlopen(req, timeout=60) as resp:
        cache_file.write_bytes(resp.read())

    return cache_file


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
    release = fetch_latest_release()
    asset_url = get_asset_url(release, WINDOWS_ASSET_NAME)
    if not asset_url:
        raise RuntimeError("Windows asset rathole tidak ditemukan pada release terbaru")

    windows_zip = cached_download(asset_url, cache_dir() / WINDOWS_ASSET_NAME)

    bundle_name = f"windows-client-{timestamp_slug()}.zip"
    out_path = bundles_dir() / bundle_name

    temp_dir = Path(tempfile.mkdtemp(prefix="easy-rathole-win-"))
    try:
        with zipfile.ZipFile(windows_zip, "r") as zf:
            with zf.open("rathole.exe", "r") as src:
                (temp_dir / "rathole.exe").write_bytes(src.read())

        (temp_dir / "client.toml").write_text(render_client_toml(state, token), encoding="utf-8")

        install_ps1_tpl = resources_dir() / "assets/windows/install-service.ps1.tpl"
        uninstall_ps1_tpl = resources_dir() / "assets/windows/uninstall-service.ps1.tpl"

        (temp_dir / "install-service.ps1").write_text(
            render_template(install_ps1_tpl, {"WINDOWS_SERVICE_NAME": WINDOWS_SERVICE_NAME}),
            encoding="utf-8",
        )
        (temp_dir / "uninstall-service.ps1").write_text(
            render_template(uninstall_ps1_tpl, {"WINDOWS_SERVICE_NAME": WINDOWS_SERVICE_NAME}),
            encoding="utf-8",
        )

        (temp_dir / "README.txt").write_text(
            "\n".join(
                [
                    "Easy Rathole Windows Client",
                    "",
                    "1) Extract this ZIP.",
                    "2) Right click install-service.ps1, Run as Administrator.",
                    "3) Service will auto start on boot.",
                    "4) To remove service, run uninstall-service.ps1 as Administrator.",
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
                    "Easy Rathole Linux Client",
                    "",
                    "1) Unzip this package on Linux client machine.",
                    "2) Run: sudo ./install-client.sh",
                    "3) Service will auto start on boot.",
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
