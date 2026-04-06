from __future__ import annotations

import os
import shutil
import tempfile
import zipfile
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

WINDOWS_BINARY_NAME = "ipos5-rathole.exe"
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
    nssm_exe = resources_dir() / f"assets/windows/{WINDOWS_NSSM_NAME}"
    require_file(windows_bin, WINDOWS_BINARY_NAME)
    require_file(nssm_exe, WINDOWS_NSSM_NAME)

    bundle_name = f"windows-client-{timestamp_slug()}.zip"
    out_path = bundles_dir() / bundle_name

    temp_dir = Path(tempfile.mkdtemp(prefix="easy-rathole-win-"))
    try:
        shutil.copy2(windows_bin, temp_dir / WINDOWS_BINARY_NAME)

        (temp_dir / "client.toml").write_text(render_client_toml(state, token), encoding="utf-8")

        install_ps1_tpl = resources_dir() / "assets/windows/install-service.ps1.tpl"
        uninstall_ps1_tpl = resources_dir() / "assets/windows/uninstall-service.ps1.tpl"
        install_cmd_tpl = resources_dir() / "assets/windows/install-service.cmd.tpl"
        uninstall_cmd_tpl = resources_dir() / "assets/windows/uninstall-service.cmd.tpl"
        setup_cmd_tpl = resources_dir() / "assets/windows/setup-client.cmd.tpl"
        require_file(install_ps1_tpl, "install-service.ps1.tpl")
        require_file(uninstall_ps1_tpl, "uninstall-service.ps1.tpl")
        require_file(install_cmd_tpl, "install-service.cmd.tpl")
        require_file(uninstall_cmd_tpl, "uninstall-service.cmd.tpl")
        require_file(setup_cmd_tpl, "setup-client.cmd.tpl")

        (temp_dir / "install-service.ps1").write_text(
            render_template(install_ps1_tpl, {"WINDOWS_SERVICE_NAME": WINDOWS_SERVICE_NAME}),
            encoding="utf-8",
        )
        (temp_dir / "uninstall-service.ps1").write_text(
            render_template(uninstall_ps1_tpl, {"WINDOWS_SERVICE_NAME": WINDOWS_SERVICE_NAME}),
            encoding="utf-8",
        )
        (temp_dir / "install-service.cmd").write_text(
            render_template(install_cmd_tpl, {}),
            encoding="utf-8",
        )
        (temp_dir / "uninstall-service.cmd").write_text(
            render_template(uninstall_cmd_tpl, {}),
            encoding="utf-8",
        )
        (temp_dir / "setup-client.cmd").write_text(
            render_template(setup_cmd_tpl, {"WINDOWS_SERVICE_NAME": WINDOWS_SERVICE_NAME}),
            encoding="utf-8",
        )
        shutil.copy2(nssm_exe, temp_dir / WINDOWS_NSSM_NAME)

        (temp_dir / "README.txt").write_text(
            "\n".join(
                [
                    "Easy Rathole Windows Client",
                    "",
                    "1) Extract this ZIP.",
                    "2) Double-click setup-client.cmd (auto ask Administrator/UAC).",
                    "3) Service will auto start on boot.",
                    f"   (binary included: {WINDOWS_BINARY_NAME})",
                    "   (nssm.exe already included in this package)",
                    "4) Advanced/manual: install-service.cmd and uninstall-service.cmd are still available.",
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
