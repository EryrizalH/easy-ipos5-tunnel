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
        ):
            (assets / name).write_bytes(b"x")

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

    def test_db_sync_status_label_uses_database_registry_errors(self) -> None:
        db_sync = {
            "enabled": True,
            "bucardo_configured": True,
            "databases": [
                {"name": "store_a", "status": "synced"},
                {
                    "name": "store_b",
                    "status": "error",
                    "phase": "repair_metadata",
                    "last_error": "validate failed",
                    "last_error_detail": "relation bucardo.bucardo_truncate_trigger does not exist",
                    "unsupported_tables": ["vps:public.sales"],
                },
            ],
        }

        self.assertEqual(main.db_sync_status_label(db_sync), "error")
        self.assertEqual(main.db_sync_summary(db_sync)["error"], 1)
        rows = main.db_sync_database_rows(db_sync)
        self.assertEqual(rows[1]["phase"], "repair_metadata")
        self.assertIn("truncate_trigger", rows[1]["last_error_detail"])
        self.assertEqual(rows[1]["unsupported_tables"], ["vps:public.sales"])

    def test_get_db_sync_debug_returns_registry(self) -> None:
        save_state(
            {
                "db_sync_mode": {
                    "enabled": True,
                    "bucardo_configured": True,
                    "last_discovery_at": "2026-06-10T00:00:00Z",
                    "databases": [
                        {
                            "name": "store_a",
                            "status": "error",
                            "phase": "preflight_database",
                            "sync_name": "ipos5_2way_store_a",
                            "last_error": "unsupported table",
                            "last_error_detail": "missing primary key",
                            "unsupported_tables": ["private:public.items"],
                        }
                    ],
                }
            },
            self.state_path,
        )

        payload = main.get_db_sync_debug(_="admin")

        self.assertEqual(payload["status"], "error")
        self.assertEqual(payload["summary"]["error"], 1)
        self.assertEqual(payload["databases"][0]["name"], "store_a")
        self.assertEqual(payload["databases"][0]["phase"], "preflight_database")
        self.assertEqual(payload["databases"][0]["last_error_detail"], "missing primary key")
        self.assertEqual(payload["databases"][0]["unsupported_tables"], ["private:public.items"])

    @patch("socket.create_connection")
    @patch("app.main.run_db_sync_finalize")
    def test_auto_finalize_callback_triggers_finalize_when_reachable(self, mock_finalize, mock_connect) -> None:
        save_state(
            {
                "db_sync_mode": {
                    "enabled": True,
                    "initial_clone_done": False,
                    "bucardo_configured": False,
                }
            },
            self.state_path,
        )
        main.last_finalize_attempt = 0.0
        
        main.auto_finalize_callback()
        
        mock_connect.assert_called_once_with(("127.0.0.1", 5445), timeout=0.5)
        mock_finalize.assert_called_once()

    @patch("socket.create_connection")
    @patch("app.main.run_db_sync_finalize")
    def test_auto_finalize_callback_honors_cooldown(self, mock_finalize, mock_connect) -> None:
        save_state(
            {
                "db_sync_mode": {
                    "enabled": True,
                    "initial_clone_done": False,
                    "bucardo_configured": False,
                    "private_db_tunnel_addr": "127.0.0.1:5444",
                }
            },
            self.state_path,
        )
        main.last_finalize_attempt = main.time.time() - 5.0 # only 5s ago, cooldown is 30s
        
        main.auto_finalize_callback()
        
        mock_connect.assert_not_called()
        mock_finalize.assert_not_called()

    @patch("app.main.subprocess.run")
    def test_get_debug_logs_success(self, mock_run) -> None:
        mock_run.return_value = main.subprocess.CompletedProcess(
            args=["journalctl"], returncode=0, stdout="rathole log lines", stderr=""
        )
        
        response = main.get_debug_logs(service="rathole", _="admin")
        self.assertEqual(response["service"], "rathole")
        self.assertEqual(response["logs"], "rathole log lines")

    def test_get_debug_logs_invalid_service(self) -> None:
        with self.assertRaises(HTTPException) as ctx:
            main.get_debug_logs(service="invalid_service", _="admin")
        self.assertEqual(ctx.exception.status_code, 400)

    @patch("app.main.subprocess.run")
    def test_get_bucardo_status(self, mock_run) -> None:
        mock_run.return_value = main.subprocess.CompletedProcess(
            args=["bucardo"], returncode=0, stdout="bucardo active", stderr=""
        )
        
        response = main.get_bucardo_status(_="admin")
        self.assertEqual(response["status"], "bucardo active")
        self.assertEqual(response["sync"], "bucardo active")


if __name__ == "__main__":
    unittest.main()
