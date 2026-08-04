from __future__ import annotations

import gc
import os
import tempfile
import unittest
import zipfile
from pathlib import Path

os.environ.setdefault("NUSA_TUNNEL_PG_MONITOR_ENABLED", "0")

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
        self.db_path = self.root / "state" / "nusatunnel.db"
        self.state_path = self.root / "state" / "install-state.json"
        self.resources.mkdir(parents=True, exist_ok=True)
        self.bundles.mkdir(parents=True, exist_ok=True)

        self._old_env = {
            "NUSA_TUNNEL_DB_PATH": os.environ.get("NUSA_TUNNEL_DB_PATH"),
            "NUSA_TUNNEL_STATE_FILE": os.environ.get("NUSA_TUNNEL_STATE_FILE"),
            "NUSA_TUNNEL_RESOURCES_DIR": os.environ.get("NUSA_TUNNEL_RESOURCES_DIR"),
            "NUSA_TUNNEL_BUNDLES_DIR": os.environ.get("NUSA_TUNNEL_BUNDLES_DIR"),
        }
        os.environ["NUSA_TUNNEL_DB_PATH"] = str(self.db_path)
        os.environ["NUSA_TUNNEL_STATE_FILE"] = str(self.state_path)
        os.environ["NUSA_TUNNEL_RESOURCES_DIR"] = str(self.resources)
        os.environ["NUSA_TUNNEL_BUNDLES_DIR"] = str(self.bundles)
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
        installer = assets / bundle_service.WINDOWS_UNIFIED_NAME
        installer.write_bytes(b"MZstub")
        with zipfile.ZipFile(installer, "a") as archive:
            for name in bundle_service.WINDOWS_EMBEDDED_RUNTIME_NAMES:
                archive.writestr(name, b"x")

    def write_state_and_db(self, *, token: str) -> None:
        init_db(str(self.db_path), "admin", "secret123", token)
        save_state(
            {
                "public_ip": "10.10.10.10",
                "rathole_control_port": 443,
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
            self.assertEqual(names, {bundle_service.WINDOWS_UNIFIED_NAME, "client.toml"})

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
        self.assertIn("tidak memakai nusatunnel-gui.exe", detail)


if __name__ == "__main__":
    unittest.main()
