from __future__ import annotations

import os
import unittest
from unittest import mock

from app.services import postgres_monitor_service as svc


class PostgresMonitorServiceTest(unittest.TestCase):
    def test_missing_credentials_disable_monitor_without_secret_defaults(self) -> None:
        with mock.patch.dict(os.environ, {}, clear=True):
            cfg = svc.read_monitor_config()
        self.assertFalse(cfg["enabled"])

    def test_password_characters_are_preserved_in_dsn(self) -> None:
        from psycopg2.extensions import parse_dsn
        password = " 'quote\\ dollar$ # unicode-雪 "
        with mock.patch.dict(os.environ, {
            "NUSA_TUNNEL_PG_MONITOR_USER": "test-user",
            "NUSA_TUNNEL_PG_MONITOR_PASSWORD": password,
        }, clear=True):
            cfg = svc.read_monitor_config()
        self.assertTrue(cfg["enabled"])
        self.assertEqual(parse_dsn(cfg["dsn"])["password"], password)

    def test_classify_status_thresholds(self) -> None:
        self.assertEqual(svc.classify_status(50.0, ""), "Healthy")
        self.assertEqual(svc.classify_status(150.0, ""), "Warning")
        self.assertEqual(svc.classify_status(350.0, ""), "Critical")
        self.assertEqual(svc.classify_status(None, ""), "Unknown")
        self.assertEqual(svc.classify_status(10.0, "boom"), "Critical")

    def test_read_monitor_config_invalid_interval_fallback(self) -> None:
        env = {
            "NUSA_TUNNEL_PG_MONITOR_ENABLED": "1",
            "NUSA_TUNNEL_PG_MONITOR_INTERVAL_SEC": "invalid",
            "NUSA_TUNNEL_PG_MONITOR_DSN": "host=127.0.0.1 port=5444 dbname=postgres user=test password=test",
        }
        with mock.patch.dict(os.environ, env, clear=False):
            cfg = svc.read_monitor_config()
        self.assertTrue(cfg["enabled"])
        self.assertEqual(cfg["interval_sec"], 5)
        self.assertIn("host=127.0.0.1", cfg["dsn"])


if __name__ == "__main__":
    unittest.main()
