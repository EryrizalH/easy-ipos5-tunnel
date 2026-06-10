from __future__ import annotations

import os
import tempfile
import unittest
from unittest import mock
from pathlib import Path

from app.services import postgres_monitor_service as svc


class PostgresMonitorServiceTest(unittest.TestCase):
    def test_classify_status_thresholds(self) -> None:
        self.assertEqual(svc.classify_status(50.0, ""), "Healthy")
        self.assertEqual(svc.classify_status(150.0, ""), "Warning")
        self.assertEqual(svc.classify_status(350.0, ""), "Critical")
        self.assertEqual(svc.classify_status(None, ""), "Unknown")
        self.assertEqual(svc.classify_status(10.0, "boom"), "Critical")

    def test_read_monitor_config_invalid_interval_fallback(self) -> None:
        env = {
            "EASY_RATHOLE_PG_MONITOR_ENABLED": "1",
            "EASY_RATHOLE_PG_MONITOR_INTERVAL_SEC": "invalid",
            "EASY_RATHOLE_PG_MONITOR_DSN": "host=127.0.0.1 port=5444 dbname=postgres user=test password=test",
        }
        with mock.patch.dict(os.environ, env, clear=False):
            cfg = svc.read_monitor_config()
        self.assertTrue(cfg["enabled"])
        self.assertEqual(cfg["interval_sec"], 5)
        self.assertIn("host=127.0.0.1", cfg["dsn"])

    def test_resolve_default_monitor_port_uses_vps_db_when_sync_enabled(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            state_path = Path(tmp) / "install-state.json"
            state_path.write_text(
                """
                {
                  "db_sync_mode": {
                    "enabled": true,
                    "vps_db_addr": "127.0.0.1:5444",
                    "private_db_tunnel_addr": "127.0.0.1:5445"
                  },
                  "service_ports": [
                    {"name": "db", "remote_bind_port": 5445}
                  ]
                }
                """,
                encoding="utf-8",
            )
            with mock.patch.dict(os.environ, {"EASY_RATHOLE_STATE_FILE": str(state_path)}, clear=False):
                self.assertEqual(svc.resolve_default_monitor_port(), 5444)


if __name__ == "__main__":
    unittest.main()
