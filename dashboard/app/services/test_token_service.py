from __future__ import annotations

import unittest

from app.services.token_service import build_server_config


class TokenServiceTest(unittest.TestCase):
    def test_build_server_config_uses_service_bind_addr(self) -> None:
        config = build_server_config(
            control_port=2333,
            token="demo-token",
            service_ports=[
                {
                    "service_key": "port_5445",
                    "protocol": "tcp",
                    "bind_addr": "127.0.0.1",
                    "remote_bind_port": 5445,
                },
                {
                    "service_key": "port_5480",
                    "protocol": "tcp",
                    "bind_addr": "0.0.0.0",
                    "remote_bind_port": 5480,
                },
            ],
        )

        self.assertIn('bind_addr = "127.0.0.1:5445"', config)
        self.assertIn('bind_addr = "0.0.0.0:5480"', config)


if __name__ == "__main__":
    unittest.main()
