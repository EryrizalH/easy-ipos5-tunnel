from __future__ import annotations

import gc
import os
import tempfile
import unittest
import zipfile
from pathlib import Path
from unittest.mock import patch

os.environ.setdefault("EASY_RATHOLE_PG_MONITOR_ENABLED", "0")

from fastapi import HTTPException

from app import main
from app.db import connect, init_db
from app.services import bundle_service
from app.state import save_state


class DashboardDownloadWindows7Test(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.addCleanup(gc.collect)
        self.root = Path(self._tmp.name)
        self.resources = self.root / "resources"
        self.bundles = self.root / "bundles"
        self.db_path = self.root / "state" / "easy-rathole.db"
        self.state_path = self.root / "state" / "install-state.json"
        self.resources.mkdir(parents=True, exist_ok=True)
        self.bundles.mkdir(parents=True, exist_ok=True)

        self._old_env = {
            "EASY_RATHOLE_DB_PATH": os.environ.get("EASY_RATHOLE_DB_PATH"),
            "EASY_RATHOLE_STATE_FILE": os.environ.get("EASY_RATHOLE_STATE_FILE"),
            "EASY_RATHOLE_RESOURCES_DIR": os.environ.get("EASY_RATHOLE_RESOURCES_DIR"),
            "EASY_RATHOLE_BUNDLES_DIR": os.environ.get("EASY_RATHOLE_BUNDLES_DIR"),
            "EASY_RATHOLE_DB_SYNC_FINALIZE_SCRIPT": os.environ.get("EASY_RATHOLE_DB_SYNC_FINALIZE_SCRIPT"),
        }
        os.environ["EASY_RATHOLE_DB_PATH"] = str(self.db_path)
        os.environ["EASY_RATHOLE_STATE_FILE"] = str(self.state_path)
        os.environ["EASY_RATHOLE_RESOURCES_DIR"] = str(self.resources)
        os.environ["EASY_RATHOLE_BUNDLES_DIR"] = str(self.bundles)
        self.addCleanup(self.restore_env)

    def restore_env(self) -> None:
        for key, value in self._old_env.items():
            if value is None:
                os.environ.pop(key, None)
            else:
                os.environ[key] = value

    def write_common_resources(self) -> None:
        templates = self.resources / "templates" / "rathole"
        templates.mkdir(parents=True, exist_ok=True)
        (templates / "client.toml.tpl").write_text(
            "remote_addr={{SERVER_ADDR}}:{{RATHOLE_CONTROL_PORT}}\n"
            "db={{DB_SERVICE_KEY}} {{DB_CLIENT_LOCAL_ADDR}}\n",
            encoding="utf-8",
        )

    def write_windows7_assets(self) -> None:
        assets = self.resources / "assets" / "windows7"
        assets.mkdir(parents=True, exist_ok=True)
        for name in (
            bundle_service.WINDOWS_SERVICE_WRAPPER_NAME,
            bundle_service.WINDOWS_RATHOLE_BINARY_NAME,
            bundle_service.WINDOWS_UNIFIED_NAME,
            bundle_service.WINDOWS_NSSM_NAME,
            bundle_service.WINDOWS_PGBOUNCER_BINARY_NAME,
            bundle_service.WINDOWS_PGBOUNCER_LIBEVENT_NAME,
            bundle_service.WINDOWS_PGBOUNCER_LIBSSL_NAME,
            bundle_service.WINDOWS_PGBOUNCER_LIBCRYPTO_NAME,
            bundle_service.WINDOWS_PGBOUNCER_LIBWINPTH_NAME,
            bundle_service.WINDOWS_PGBOUNCER_USERLIST_NAME,
        ):
            (assets / name).write_bytes(b"x")
        (assets / "pgbouncer.ini.tpl").write_text("[databases]\n", encoding="utf-8")

    def write_state_and_db(self, *, token: str) -> None:
        init_db(str(self.db_path), "admin", "secret123", token)
        save_state(
            {
                "public_ip": "10.10.10.10",
                "rathole_control_port": 2333,
            },
            self.state_path,
        )

    def test_download_windows7_success(self) -> None:
        self.write_common_resources()
        self.write_windows7_assets()
        self.write_state_and_db(token="demo-token")

        conn = connect(str(self.db_path))
        try:
            response = main.download_windows7(_="admin", conn=conn)
        finally:
            conn.close()

        self.assertEqual(response.status_code, 200)
        archive_path = Path(response.path)
        self.assertTrue(archive_path.name.startswith("windows7-client-"))

        self.assertTrue(archive_path.exists())
        with zipfile.ZipFile(archive_path) as zf:
            names = set(zf.namelist())
            self.assertIn("client.toml", names)
            self.assertNotIn(bundle_service.WINDOWS_GUI_BINARY_NAME, names)

    def test_download_windows7_rejects_when_token_missing(self) -> None:
        self.write_common_resources()
        self.write_windows7_assets()
        self.write_state_and_db(token="")

        conn = connect(str(self.db_path))
        try:
            with self.assertRaises(HTTPException) as ctx:
                main.download_windows7(_="admin", conn=conn)
        finally:
            conn.close()
        self.assertEqual(ctx.exception.status_code, 400)
        self.assertEqual(ctx.exception.detail, "Token belum diset")

    def test_download_windows7_reports_missing_assets(self) -> None:
        self.write_common_resources()
        self.write_state_and_db(token="demo-token")

        conn = connect(str(self.db_path))
        try:
            with self.assertRaises(HTTPException) as ctx:
                main.download_windows7(_="admin", conn=conn)
        finally:
            conn.close()

        self.assertEqual(ctx.exception.status_code, 400)
        detail = str(ctx.exception.detail)
        self.assertIn("Windows 7", detail)
        self.assertIn("assets/windows7/setup.exe", detail)
        self.assertIn("tidak memakai ipos5-rathole-gui.exe", detail)

    def write_finalize_script(self) -> Path:
        script = self.root / "install_db_sync_bucardo.sh"
        script.write_text("#!/usr/bin/env bash\nexit 0\n", encoding="utf-8")
        os.environ["EASY_RATHOLE_DB_SYNC_FINALIZE_SCRIPT"] = str(script)
        return script

    def test_finalize_db_sync_rejects_when_disabled(self) -> None:
        save_state({}, self.state_path)

        response = main.finalize_db_sync(_="admin")

        self.assertEqual(response.status_code, 303)
        self.assertIn("DB+sync+belum+aktif", response.headers["location"])

    def test_finalize_db_sync_reports_configured_after_success(self) -> None:
        self.write_finalize_script()
        save_state(
            {
                "db_sync_mode": {
                    "enabled": True,
                    "initial_clone_done": False,
                    "bucardo_configured": False,
                    "waiting_for_client": False,
                }
            },
            self.state_path,
        )

        def fake_run(*args, **kwargs):
            save_state(
                {
                    "db_sync_mode": {
                        "enabled": True,
                        "initial_clone_done": True,
                        "bucardo_configured": True,
                        "waiting_for_client": False,
                    }
                },
                self.state_path,
            )
            return main.subprocess.CompletedProcess(args=args[0], returncode=0, stdout="ok", stderr="")

        with patch.object(main.subprocess, "run", side_effect=fake_run):
            response = main.finalize_db_sync(_="admin")

        self.assertEqual(response.status_code, 303)
        self.assertIn("Finalisasi+DB+sync+berhasil", response.headers["location"])

    def test_finalize_db_sync_reports_waiting_client_after_success(self) -> None:
        self.write_finalize_script()
        save_state(
            {
                "db_sync_mode": {
                    "enabled": True,
                    "initial_clone_done": False,
                    "bucardo_configured": False,
                    "waiting_for_client": False,
                }
            },
            self.state_path,
        )

        def fake_run(*args, **kwargs):
            save_state(
                {
                    "db_sync_mode": {
                        "enabled": True,
                        "initial_clone_done": False,
                        "bucardo_configured": False,
                        "waiting_for_client": True,
                    }
                },
                self.state_path,
            )
            return main.subprocess.CompletedProcess(args=args[0], returncode=0, stdout="waiting", stderr="")

        with patch.object(main.subprocess, "run", side_effect=fake_run):
            response = main.finalize_db_sync(_="admin")

        self.assertEqual(response.status_code, 303)
        self.assertIn("menunggu+client", response.headers["location"])

    def test_finalize_db_sync_reports_subprocess_failure(self) -> None:
        self.write_finalize_script()
        save_state(
            {
                "db_sync_mode": {
                    "enabled": True,
                    "initial_clone_done": False,
                    "bucardo_configured": False,
                    "waiting_for_client": False,
                }
            },
            self.state_path,
        )

        completed = main.subprocess.CompletedProcess(
            args=["bash", "install_db_sync_bucardo.sh"],
            returncode=1,
            stdout="",
            stderr="private db error",
        )
        with patch.object(main.subprocess, "run", return_value=completed):
            response = main.finalize_db_sync(_="admin")

        self.assertEqual(response.status_code, 303)
        self.assertIn("Finalisasi+DB+sync+gagal", response.headers["location"])


if __name__ == "__main__":
    unittest.main()
