from __future__ import annotations

import functools
import http.server
import io
import json
import os
import shutil
import subprocess
import sys
import tarfile
import tempfile
import threading
import unittest
import zipfile
from pathlib import Path
from unittest import mock

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))
import release
from generate_release_credentials import generate

BASH = os.environ.get("NUSA_TEST_BASH") or shutil.which("bash")
TAG = "v1.2.3"
REPO = "example/tunnel"


class QuietServer(http.server.SimpleHTTPRequestHandler):
    def log_message(self, *_):
        pass


class ReleaseTest(unittest.TestCase):
    @unittest.skipUnless(BASH, "Bash tidak tersedia")
    def test_stdin_entrypoint_runs_without_bash_source(self):
        source = (ROOT / "public-install.sh").read_text(encoding="utf-8")
        body, guard = source.rsplit("\nif [[", 1)
        # Replace only the installation function with a spy, leaving the real
        # entrypoint guard unchanged. Never execute an actual system installer.
        script = body + '\nmain() { printf ENTRYPOINT_CALLED; }\nif [[' + guard
        result = subprocess.run([BASH, "--noprofile", "--norc"], input=script,
                                text=True, capture_output=True, timeout=10)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, "ENTRYPOINT_CALLED")

    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="nusa release test ")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.source = self.root / "source"
        for name in release.DEPLOY_FILES + ("public-install.sh",):
            target = self.source / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / name, target)
        (self.source / "dashboard/app").mkdir(parents=True, exist_ok=True)
        (self.source / "dashboard/app/main.py").write_text("# public application\n")
        (self.source / "dashboard/app/test_private.py").write_text("# excluded test\n")
        (self.source / ".env").write_text("DO_NOT_SHIP=fixture\n")
        for name in release.SETUPS:
            path = self.source / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(b"MZfixture")
            with zipfile.ZipFile(path, "a") as archive:
                for runtime in release.RUNTIME | ({"nusatunnel-gui.exe"} if "windows7" not in name else set()):
                    archive.writestr(runtime, "fixture runtime")
        assets = {}
        for name in ("rathole-x86_64-unknown-linux-gnu.zip", "rathole-aarch64-unknown-linux-musl.zip"):
            path = self.source / "assets/linux" / name
            with zipfile.ZipFile(path, "w") as archive:
                archive.writestr("rathole", "fixture runtime")
            assets[name] = {"url": "https://example.invalid/" + name, "sha256": release.digest(path)}
        (self.source / "scripts/release-dependencies.json").write_text(json.dumps({
            "rathole_version": "v0.5.0", "linux_assets": assets, "windows_vendor": [],
        }))
        self.output = self.root / "http" / TAG
        release.package(self.source, self.output, TAG, REPO, "a" * 40)
        self.marker = self.root / "installer-called"

    def serve(self):
        latest = self.root / "http/latest"
        latest.write_text(json.dumps({"tag_name": TAG, "draft": False, "prerelease": False}))
        handler = functools.partial(QuietServer, directory=str(self.root / "http"))
        server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        return f"http://127.0.0.1:{server.server_port}"

    def bootstrap(self, pinned=True, arch="x86_64"):
        if not BASH:
            self.skipTest("Bash tidak tersedia")
        base = self.serve()
        env = dict(os.environ)
        for key in ("NUSA_TUNNEL_VERSION", "REPO_BRANCH", "NUSA_TUNNEL_ROOT"):
            env.pop(key, None)
        env.update({
            "REPO_URL": "https://github.com/" + REPO,
            "REPO_BASE_DIR": (self.root / "deployment").as_posix(),
            "NUSA_TUNNEL_RELEASE_API_URL": base + "/latest", "NUSA_TUNNEL_DOWNLOAD_BASE": base,
            "NUSA_TEST_PYTHON": Path(sys.executable).as_posix(),
            "NUSA_TEST_BOOTSTRAP": (ROOT / "public-install.sh").as_posix(),
            "NUSA_TEST_MARKER": self.marker.as_posix(), "NUSA_TEST_ARCH": arch,
        })
        if pinned:
            env["NUSA_TUNNEL_VERSION"] = TAG
        script = '''
source "$NUSA_TEST_BOOTSTRAP"
require_root() { :; }
python3() { "$NUSA_TEST_PYTHON" "$@"; }
uname() { printf '%s\\n' "$NUSA_TEST_ARCH"; }
bash() { printf '%s' "$NUSA_TUNNEL_VERSION" > "$NUSA_TEST_MARKER"; printf '%s' "$NUSA_TUNNEL_ROOT" > "$NUSA_TEST_MARKER.root"; }
main
'''
        return subprocess.run([BASH, "--noprofile", "--norc", "-c", script], env=env, capture_output=True, text=True, timeout=30)

    def update_sums(self):
        (self.output / "SHA256SUMS").write_text("".join(
            f"{release.digest(path)}  {path.name}\n" for path in sorted(self.output.iterdir()) if path.name != "SHA256SUMS"
        ))

    def test_allowlist_and_checksums(self):
        with tarfile.open(self.output / "nusatunnel-server.tar.gz") as archive:
            names = archive.getnames()
        self.assertIn("assets/windows/setup.exe", names)
        self.assertNotIn(".env", names)
        self.assertFalse(any("test_" in name or "client.toml" == name or "lock-ipos" in name for name in names))
        for line in (self.output / "SHA256SUMS").read_text().splitlines():
            expected, name = line.split("  ")
            self.assertEqual(release.digest(self.output / name), expected)

    def test_latest_and_reinstall_preserve_state(self):
        state = self.root / "deployment/state/user-state"
        state.parent.mkdir(parents=True)
        state.write_text("keep")
        old = self.root / "deployment/src/old-checkout"
        old.parent.mkdir(parents=True)
        old.write_text("keep")
        for pinned in (False, True):
            result = self.bootstrap(pinned=pinned)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.marker.read_text(), TAG)
        self.assertEqual(Path(str(self.marker) + ".root").read_text(), "/opt/nusatunnel")
        self.assertEqual(state.read_text(), "keep")
        self.assertEqual(old.read_text(), "keep")
        self.assertEqual(len(list((self.root / "deployment/releases" / TAG).glob("install-*"))), 2)

    def test_missing_asset_stops_before_installer(self):
        (self.output / "nusatunnel-server.tar.gz").unlink()
        self.assertNotEqual(self.bootstrap().returncode, 0)
        self.assertFalse(self.marker.exists())

    def test_checksum_mismatch_stops_before_installer(self):
        with (self.output / "nusatunnel-server.tar.gz").open("ab") as stream:
            stream.write(b"tamper")
        self.assertNotEqual(self.bootstrap().returncode, 0)
        self.assertFalse(self.marker.exists())

    def test_wrong_manifest_stops_before_installer(self):
        path = self.output / "release.json"
        data = json.loads(path.read_text())
        data["version"] = "v9.9.9"
        path.write_text(json.dumps(data))
        self.update_sums()
        self.assertNotEqual(self.bootstrap().returncode, 0)
        self.assertFalse(self.marker.exists())

    def test_unsupported_arch_stops_before_installer(self):
        self.assertNotEqual(self.bootstrap(arch="sparc").returncode, 0)
        self.assertFalse(self.marker.exists())

    def test_archive_traversal_is_rejected(self):
        with tarfile.open(self.output / "nusatunnel-server.tar.gz", "w:gz") as archive:
            member = tarfile.TarInfo("../escape")
            member.size = 1
            archive.addfile(member, io.BytesIO(b"x"))
        self.update_sums()
        self.assertNotEqual(self.bootstrap().returncode, 0)
        self.assertFalse(self.marker.exists())

    def test_win7_rejects_gui_payload(self):
        with self.assertRaises(ValueError):
            release.check_setup(self.source / "assets/windows/setup.exe", False)


class CredentialTest(unittest.TestCase):
    def test_generator_requires_values_and_keeps_special_characters(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "lock-ipos/internal/db").mkdir(parents=True)
            with mock.patch.dict(os.environ, {}, clear=True):
                with self.assertRaises(SystemExit):
                    generate(root)
            password = 'quote" slash\\ dollar$ unicode雪😀'
            with mock.patch.dict(os.environ, {"IPOS5_DB_USERNAME": "dummy", "IPOS5_DB_PASSWORD": password}):
                generate(root)
            content = (root / "lock-ipos/internal/db/credentials_generated.go").read_text(encoding="utf-8")
            self.assertIn(json.dumps(password, ensure_ascii=False), content)

    def test_monitor_file_permissions_and_escaping(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "config/monitor.env"
            password = ' quote" slash\\ dollar$ # unicode雪 '
            with mock.patch.dict(os.environ, {"NUSA_TUNNEL_PG_MONITOR_USER": "dummy", "NUSA_TUNNEL_PG_MONITOR_PASSWORD": password}):
                release.write_monitor_env(path)
            content = path.read_text(encoding="utf-8")
            release.check_monitor_env(path)
            self.assertIn('quote\\" slash\\\\ dollar$', content)
            if os.name != "nt":
                self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            with mock.patch.dict(os.environ, {"NUSA_TUNNEL_PG_MONITOR_PASSWORD": "bad\nvalue"}):
                with self.assertRaises(ValueError):
                    release.write_monitor_env(path)
            self.assertEqual(path.read_text(encoding="utf-8"), content)
            path.write_text('NUSA_TUNNEL_PG_MONITOR_USER="dummy"\n')
            with self.assertRaises(ValueError):
                release.check_monitor_env(path)

    @unittest.skipUnless(BASH and os.name != "nt", "Requires Unix controlling terminal")
    def test_tty_prompt_hides_password_with_pipe_stdin(self):
        import pty
        import select
        import time
        pid, fd = pty.fork()
        if pid == 0:
            os.environ["NUSA_TUNNEL_CONFIG_DIR"] = "/nonexistent-nusatunnel-fixture"
            for name in ("NUSA_TUNNEL_PG_MONITOR_USER", "NUSA_TUNNEL_PG_MONITOR_PASSWORD"):
                os.environ.pop(name, None)
            script = f'''source '{ROOT}/scripts/lib/monitor_credentials.sh'
fail() {{ exit 1; }}
collect_monitor_credentials </dev/null
printf 'COLLECTED'
'''
            os.execl(BASH, BASH, "-euc", script)
        received = b""
        sent_user = sent_password = False
        deadline = time.monotonic() + 10
        try:
            while time.monotonic() < deadline:
                if select.select([fd], [], [], .1)[0]:
                    try:
                        chunk = os.read(fd, 4096)
                    except OSError:
                        break
                    if not chunk:
                        break
                    received += chunk
                    if b"Username database" in received and not sent_user:
                        os.write(fd, b"dummy\n")
                        sent_user = True
                    if b"Password database" in received and not sent_password:
                        os.write(fd, b"hidden-fixture-password\n")
                        sent_password = True
                    if b"COLLECTED" in received:
                        break
            self.assertIn(b"COLLECTED", received)
            self.assertNotIn(b"hidden-fixture-password", received)
        finally:
            os.close(fd)
            try:
                os.kill(pid, 15)
            except ProcessLookupError:
                pass
            os.waitpid(pid, 0)

    @unittest.skipUnless(BASH and os.name != "nt", "Requires Unix permission checks")
    def test_no_tty_missing_credentials_fails_and_existing_file_reused(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            env = dict(os.environ, NUSA_TUNNEL_CONFIG_DIR=temporary)
            for name in ("NUSA_TUNNEL_PG_MONITOR_USER", "NUSA_TUNNEL_PG_MONITOR_PASSWORD"):
                env.pop(name, None)
            script = f"source '{ROOT}/scripts/lib/monitor_credentials.sh'; fail() {{ exit 1; }}; collect_monitor_credentials"
            result = subprocess.run([BASH, "-euc", script], env=env, stdin=subprocess.DEVNULL, capture_output=True, start_new_session=True)
            self.assertNotEqual(result.returncode, 0)
            path = directory / "postgres-monitor.env"
            path.write_text('NUSA_TUNNEL_PG_MONITOR_USER="dummy"\nNUSA_TUNNEL_PG_MONITOR_PASSWORD="fixture"\n')
            path.chmod(0o600)
            # CI runs unprivileged: mock ownership only, still use actual permission bits.
            script = f"source '{ROOT}/scripts/lib/monitor_credentials.sh'; fail() {{ exit 1; }}; stat() {{ printf '0:%s' \"$(/usr/bin/stat -c %a \"$3\")\"; }}; collect_monitor_credentials"
            before = path.read_bytes()
            result = subprocess.run([BASH, "-euc", script], env=env, stdin=subprocess.DEVNULL, capture_output=True, start_new_session=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(path.read_bytes(), before)
            path.chmod(0o644)
            result = subprocess.run([BASH, "-euc", script], env=env, stdin=subprocess.DEVNULL, capture_output=True, start_new_session=True)
            self.assertNotEqual(result.returncode, 0)


if __name__ == "__main__":
    unittest.main()
