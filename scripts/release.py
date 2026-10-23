"""Build and validate the public deployment allowlist; stdlib only, Python 3.11+."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import shutil
import tarfile
import tempfile
import urllib.request
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
TAG = re.compile(r"v[0-9]+\.[0-9]+\.[0-9]+\Z")
REPOSITORY = re.compile(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+\Z")
SETUPS = {
    "assets/windows/setup.exe": "nusatunnel-windows-amd64-setup.exe",
    "assets/windows7/setup.exe": "nusatunnel-windows7-amd64-setup.exe",
}
RUNTIME = {
    "nssm.exe", "nusatunnel-service.exe", "nusatunnel.exe", "pgbouncer.exe",
    "libevent-7.dll", "libssl-3-x64.dll", "libcrypto-3-x64.dll", "libwinpthread-1.dll",
}
DEPLOY_FILES = (
    "install.sh", "scripts/prepare_server.sh", "scripts/install_dashboard.sh",
    "scripts/install_rathole_server.sh", "scripts/lib/common.sh",
    "scripts/lib/release.sh", "scripts/lib/monitor_credentials.sh", "scripts/release.py",
    "dashboard/requirements.txt", "dashboard/systemd/nusatunnel-dashboard.service.tpl",
    "templates/rathole/server.toml.tpl", "templates/rathole/client.toml.tpl",
    "assets/linux/install-client.sh.tpl",
    "dashboard/app/__init__.py", "dashboard/app/auth.py", "dashboard/app/bootstrap.py",
    "dashboard/app/db.py", "dashboard/app/main.py", "dashboard/app/state.py",
    "dashboard/app/services/__init__.py", "dashboard/app/services/bundle_service.py",
    "dashboard/app/services/postgres_monitor_service.py", "dashboard/app/services/token_service.py",
    "dashboard/app/services/tunnel_ports.py", "dashboard/app/templates/dashboard.html",
)


def digest(path: Path) -> str:
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def check_file(path: Path, expected: str) -> None:
    if not path.is_file() or digest(path) != expected:
        raise ValueError(f"Checksum/file tidak valid: {path.name}")


def check_setup(path: Path, gui: bool) -> None:
    if not path.read_bytes()[:2] == b"MZ":
        raise ValueError(f"Bukan executable Windows: {path.name}")
    with zipfile.ZipFile(path) as archive:
        expected = RUNTIME | ({"nusatunnel-gui.exe"} if gui else set())
        if (set(archive.namelist()) != expected or len(archive.infolist()) != len(expected)
                or any(e.file_size <= 0 for e in archive.infolist())):
            raise ValueError(f"Payload installer tidak sesuai: {path}")
        if archive.testzip() is not None:
            raise ValueError(f"Payload installer rusak: {path}")


def metadata(root: Path) -> dict:
    data = json.loads((root / "release.json").read_text(encoding="utf-8"))
    if not TAG.fullmatch(data.get("version", "")) or not REPOSITORY.fullmatch(data.get("repository", "")):
        raise ValueError("Metadata release tidak valid")
    if not re.fullmatch(r"[0-9a-f]{40}", data.get("commit", "")):
        raise ValueError("Commit release tidak valid")
    return data


def preflight(root: Path) -> dict:
    data = metadata(root)
    for name in DEPLOY_FILES:
        if not (root / name).is_file():
            raise ValueError(f"File deployment tidak lengkap: {name}")
    for name in SETUPS:
        check_file(root / name, data["windows_setups"][name])
        check_setup(root / name, gui="windows7" not in name)
    for name, info in data["linux_assets"].items():
        if not re.fullmatch(r"rathole-(x86_64-unknown-linux-gnu|aarch64-unknown-linux-musl)\.zip", name):
            raise ValueError("Nama runtime Linux tidak valid")
        path = root / "assets/linux" / name
        check_file(path, info["sha256"])
        with zipfile.ZipFile(path) as archive:
            if "rathole" not in archive.namelist() or archive.testzip() is not None:
                raise ValueError("Runtime Linux tidak valid")
    if len(data["linux_assets"]) != 2:
        raise ValueError("Kedua arsitektur Linux wajib tersedia")
    return data


def fetch_runtime(root: Path) -> None:
    lock = json.loads((root / "scripts/release-dependencies.json").read_text(encoding="utf-8"))
    for vendor in lock["windows_vendor"]:
        check_file(root / vendor["path"], vendor["sha256"])
    for name, info in lock["linux_assets"].items():
        target = root / "assets/linux" / name
        target.parent.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory() as temporary:
            downloaded = Path(temporary) / name
            urllib.request.urlretrieve(info["url"], downloaded)
            check_file(downloaded, info["sha256"])
            shutil.copyfile(downloaded, target)


def package(root: Path, output: Path, tag: str, repository: str, commit: str) -> None:
    if not TAG.fullmatch(tag) or not REPOSITORY.fullmatch(repository) or not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise ValueError("Tag harus vX.Y.Z, repository owner/name, dan commit SHA lengkap")
    output.mkdir(parents=True, exist_ok=True)
    if any(output.iterdir()):
        raise ValueError("Folder output harus kosong agar aset lama tidak ikut release")
    lock = json.loads((root / "scripts/release-dependencies.json").read_text(encoding="utf-8"))
    data = {
        "version": tag, "repository": repository, "commit": commit,
        "rathole_version": lock["rathole_version"], "linux_assets": lock["linux_assets"],
        "windows_setups": {name: digest(root / name) for name in SETUPS},
    }
    with tempfile.TemporaryDirectory(prefix="nusatunnel-release-") as temporary:
        stage = Path(temporary)
        deploy = list(DEPLOY_FILES)
        deploy.extend(SETUPS)
        deploy.extend("assets/linux/" + name for name in data["linux_assets"])
        for name in deploy:
            target = stage / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(root / name, target)
        manifest = json.dumps(data, indent=2, sort_keys=True) + "\n"
        (stage / "release.json").write_text(manifest, encoding="utf-8")
        preflight(stage)
        with tarfile.open(output / "nusatunnel-server.tar.gz", "w:gz") as archive:
            for path in sorted(stage.rglob("*")):
                if path.is_file():
                    info = archive.gettarinfo(str(path), path.relative_to(stage).as_posix())
                    info.uid = info.gid = 0
                    info.uname = info.gname = "root"
                    info.mode = 0o755 if path.suffix == ".sh" else 0o644
                    with path.open("rb") as stream:
                        archive.addfile(info, stream)
        (output / "release.json").write_text(manifest, encoding="utf-8")
    shutil.copyfile(root / "public-install.sh", output / "public-install.sh")
    for source, name in SETUPS.items():
        shutil.copyfile(root / source, output / name)
    for name in data["linux_assets"]:
        shutil.copyfile(root / "assets/linux" / name, output / name)
    checksums = "".join(f"{digest(path)}  {path.name}\n" for path in sorted(output.iterdir()))
    (output / "SHA256SUMS").write_text(checksums, encoding="utf-8")


def write_monitor_env(path: Path) -> None:
    names = ("NUSA_TUNNEL_PG_MONITOR_USER", "NUSA_TUNNEL_PG_MONITOR_PASSWORD")
    values = [os.environ.get(name, "") for name in names]
    if not all(values) or any(any(c in value for c in "\r\n\0") for value in values):
        raise ValueError("Kredensial monitor wajib terisi tanpa karakter newline/NUL")
    def quote(value: str) -> str:
        return '"' + value.replace('\\', '\\\\').replace('"', '\\"') + '"'
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o750)
    descriptor, temporary = tempfile.mkstemp(prefix=".postgres-monitor-", dir=path.parent)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8", newline="\n") as stream:
            stream.write("".join(f"{name}={quote(value)}\n" for name, value in zip(names, values)))
        os.chmod(temporary, 0o600)
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def check_monitor_env(path: Path) -> None:
    lines = path.read_text(encoding="utf-8").splitlines()
    for key in ("NUSA_TUNNEL_PG_MONITOR_USER", "NUSA_TUNNEL_PG_MONITOR_PASSWORD"):
        values = [line[len(key) + 1:] for line in lines if line.startswith(key + "=")]
        if len(values) != 1 or not values[0] or values[0] in ('""', "''"):
            raise ValueError("Kredensial monitor belum lengkap")
        # Files written by this installer use systemd double-quoted values.
        if not re.fullmatch(r'"(?:[^"\\]|\\["\\])+"|[^\s\'"\\]+', values[0]):
            raise ValueError("Format file kredensial monitor tidak valid")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("fetch-runtime", "package", "preflight", "monitor-env", "monitor-env-check"))
    parser.add_argument("--root", type=Path, default=ROOT)
    parser.add_argument("--output", type=Path, default=ROOT / "release-dist")
    parser.add_argument("--tag", default="")
    parser.add_argument("--repository", default="EryrizalH/easy-ipos5-tunnel")
    parser.add_argument("--commit", default="")
    args = parser.parse_args()
    try:
        if args.command == "fetch-runtime":
            fetch_runtime(args.root)
        elif args.command == "package":
            package(args.root, args.output, args.tag, args.repository, args.commit)
        elif args.command == "monitor-env":
            write_monitor_env(args.output)
        elif args.command == "monitor-env-check":
            check_monitor_env(args.output)
        else:
            preflight(args.root)
    except (ValueError, KeyError, OSError, zipfile.BadZipFile) as exc:
        # Never include input credential values in diagnostics.
        if args.command.startswith("monitor-env"):
            parser.exit(1, "File kredensial monitor tidak tersedia, tidak lengkap, atau tidak valid.\n")
        parser.exit(1, f"Release tidak valid: {exc}\n")


if __name__ == "__main__":
    main()
