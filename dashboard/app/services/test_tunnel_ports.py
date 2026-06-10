from __future__ import annotations

import unittest

from app.services.tunnel_ports import exposed_ports_from_service_ports, normalize_service_ports


class TunnelPortsTest(unittest.TestCase):
    def test_normalize_defaults_db_client_local_port(self) -> None:
        rows = normalize_service_ports(None)
        by_name = {row["name"]: row for row in rows}
        self.assertEqual(by_name["db"]["remote_bind_port"], 5444)
        self.assertEqual(by_name["db"]["client_local_port"], 5444)
        self.assertEqual(by_name["db"]["bind_addr"], "0.0.0.0")
        self.assertTrue(by_name["db"]["expose_public"])

    def test_normalize_db_sync_uses_internal_tunnel_to_private_5444(self) -> None:
        rows = normalize_service_ports(
            None,
            {
                "enabled": True,
                "vps_db_addr": "127.0.0.1:5444",
                "private_db_tunnel_addr": "127.0.0.1:5445",
            },
        )
        by_name = {row["name"]: row for row in rows}
        self.assertEqual(by_name["db"]["service_key"], "port_5445")
        self.assertEqual(by_name["db"]["bind_addr"], "127.0.0.1")
        self.assertEqual(by_name["db"]["remote_bind_port"], 5445)
        self.assertEqual(by_name["db"]["client_local_addr"], "127.0.0.1:5444")
        self.assertFalse(by_name["db"]["expose_public"])
        self.assertEqual(
            exposed_ports_from_service_ports(
                rows,
                {
                    "enabled": True,
                    "vps_db_addr": "0.0.0.0:5444",
                    "private_db_tunnel_addr": "127.0.0.1:5445",
                },
            ),
            [5444, 5480, 5485],
        )

    def test_db_sync_mode_overrides_legacy_db_service_ports(self) -> None:
        rows = normalize_service_ports(
            [
                {
                    "name": "db",
                    "service_key": "port_5444",
                    "protocol": "tcp",
                    "remote_bind_port": 5444,
                    "client_local_addr": "127.0.0.1:5444",
                    "client_local_port": 5444,
                }
            ],
            {"enabled": True},
        )
        by_name = {row["name"]: row for row in rows}
        self.assertEqual(by_name["db"]["remote_bind_port"], 5445)
        self.assertEqual(by_name["db"]["client_local_addr"], "127.0.0.1:5444")

    def test_normalize_keeps_custom_extra_service(self) -> None:
        rows = normalize_service_ports(
            [
                {
                    "name": "custom_api",
                    "service_key": "port_7000",
                    "protocol": "tcp",
                    "remote_bind_port": 7000,
                    "client_local_addr": "127.0.0.1:7000",
                    "client_local_port": 7000,
                }
            ]
        )
        by_name = {row["name"]: row for row in rows}
        self.assertIn("custom_api", by_name)
        self.assertEqual(by_name["custom_api"]["remote_bind_port"], 7000)

    def test_exposed_ports_unique(self) -> None:
        ports = exposed_ports_from_service_ports(
            [
                {"remote_bind_port": 5444},
                {"remote_bind_port": 5444},
                {"remote_bind_port": 5480},
            ]
        )
        self.assertEqual(ports, [5444, 5480])


if __name__ == "__main__":
    unittest.main()
