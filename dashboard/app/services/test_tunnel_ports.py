from __future__ import annotations

import unittest

from app.services.tunnel_ports import exposed_ports_from_service_ports, normalize_service_ports


class TunnelPortsTest(unittest.TestCase):
    def test_normalize_defaults_db_client_local_port(self) -> None:
        rows = normalize_service_ports(None)
        by_name = {row["name"]: row for row in rows}
        self.assertEqual(by_name["db"]["remote_bind_port"], 5444)
        self.assertEqual(by_name["db"]["client_local_port"], 5444)

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

