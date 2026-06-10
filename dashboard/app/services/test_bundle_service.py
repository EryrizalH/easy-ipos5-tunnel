from __future__ import annotations

import json
import os
import tempfile
import unittest
import zipfile
from pathlib import Path

from app.services import bundle_service


class BundleServiceTest(unittest.TestCase):
    def test_generate_windows_bundle_includes_correct_files(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            resources = root / "resources"
            bundles = root / "bundles"
            assets = resources / "assets" / "windows"
            templates = resources / "templates" / "rathole"
            assets.mkdir(parents=True, exist_ok=True)
            templates.mkdir(parents=True, exist_ok=True)
            bundles.mkdir(parents=True, exist_ok=True)

            for name in (
                bundle_service.WINDOWS_SERVICE_WRAPPER_NAME,
                bundle_service.WINDOWS_RATHOLE_BINARY_NAME,
                bundle_service.WINDOWS_GUI_BINARY_NAME,
                bundle_service.WINDOWS_UNIFIED_NAME,
                bundle_service.WINDOWS_NSSM_NAME,
            ):
                (assets / name).write_bytes(b"x")

            (templates / "client.toml.tpl").write_text(
                "remote_addr={{SERVER_ADDR}}:{{RATHOLE_CONTROL_PORT}}\n"
                "db={{DB_SERVICE_KEY}} {{DB_CLIENT_LOCAL_ADDR}}\n",
                encoding="utf-8",
            )

            previous_resources = os.environ.get("EASY_RATHOLE_RESOURCES_DIR")
            previous_bundles = os.environ.get("EASY_RATHOLE_BUNDLES_DIR")
            os.environ["EASY_RATHOLE_RESOURCES_DIR"] = str(resources)
            os.environ["EASY_RATHOLE_BUNDLES_DIR"] = str(bundles)
            try:
                bundle_path = bundle_service.generate_windows_bundle(
                    {
                        "public_ip": "10.10.10.10",
                        "rathole_control_port": 2333,
                    },
                    token="demo-token",
                )
            finally:
                if previous_resources is None:
                    os.environ.pop("EASY_RATHOLE_RESOURCES_DIR", None)
                else:
                    os.environ["EASY_RATHOLE_RESOURCES_DIR"] = previous_resources
                if previous_bundles is None:
                    os.environ.pop("EASY_RATHOLE_BUNDLES_DIR", None)
                else:
                    os.environ["EASY_RATHOLE_BUNDLES_DIR"] = previous_bundles

            self.assertTrue(bundle_path.exists())
            with zipfile.ZipFile(bundle_path) as zf:
                names = set(zf.namelist())
                self.assertIn(bundle_service.WINDOWS_SERVICE_WRAPPER_NAME, names)
                self.assertIn(bundle_service.WINDOWS_RATHOLE_BINARY_NAME, names)
                self.assertIn(bundle_service.WINDOWS_GUI_BINARY_NAME, names)
                self.assertIn(bundle_service.WINDOWS_UNIFIED_NAME, names)
                self.assertIn(bundle_service.WINDOWS_NSSM_NAME, names)
                self.assertIn("client.toml", names)
                client_toml = zf.read("client.toml").decode("utf-8")

            self.assertIn("127.0.0.1:5444", client_toml)

    def test_render_client_toml_db_sync_targets_5444(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            resources = root / "resources"
            templates = resources / "templates" / "rathole"
            templates.mkdir(parents=True, exist_ok=True)
            (templates / "client.toml.tpl").write_text(
                "remote_addr={{SERVER_ADDR}}:{{RATHOLE_CONTROL_PORT}}\n"
                "db={{DB_SERVICE_KEY}} {{DB_CLIENT_LOCAL_ADDR}}\n"
                "http={{POS_HTTP_SERVICE_KEY}} {{POS_HTTP_CLIENT_LOCAL_ADDR}}\n",
                encoding="utf-8",
            )

            previous_resources = os.environ.get("EASY_RATHOLE_RESOURCES_DIR")
            os.environ["EASY_RATHOLE_RESOURCES_DIR"] = str(resources)
            try:
                client_toml = bundle_service.render_client_toml(
                    {
                        "public_ip": "10.10.10.10",
                        "rathole_control_port": 2333,
                        "db_sync_mode": {
                            "enabled": True,
                            "private_db_tunnel_addr": "127.0.0.1:5444",
                        },
                    },
                    token="demo-token",
                )
            finally:
                if previous_resources is None:
                    os.environ.pop("EASY_RATHOLE_RESOURCES_DIR", None)
                else:
                    os.environ["EASY_RATHOLE_RESOURCES_DIR"] = previous_resources

            self.assertIn("db=port_5445 127.0.0.1:5444", client_toml)
            self.assertIn("http=port_5480 127.0.0.1:5480", client_toml)

    def test_generate_windows7_bundle_excludes_gui_binary(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            resources = root / "resources"
            bundles = root / "bundles"
            assets = resources / "assets" / "windows7"
            templates = resources / "templates" / "rathole"
            assets.mkdir(parents=True, exist_ok=True)
            templates.mkdir(parents=True, exist_ok=True)
            bundles.mkdir(parents=True, exist_ok=True)

            for name in (
                bundle_service.WINDOWS_SERVICE_WRAPPER_NAME,
                bundle_service.WINDOWS_RATHOLE_BINARY_NAME,
                bundle_service.WINDOWS_UNIFIED_NAME,
                bundle_service.WINDOWS_NSSM_NAME,
            ):
                (assets / name).write_bytes(b"x")

            (templates / "client.toml.tpl").write_text(
                "remote_addr={{SERVER_ADDR}}:{{RATHOLE_CONTROL_PORT}}\n"
                "db={{DB_SERVICE_KEY}} {{DB_CLIENT_LOCAL_ADDR}}\n",
                encoding="utf-8",
            )

            previous_resources = os.environ.get("EASY_RATHOLE_RESOURCES_DIR")
            previous_bundles = os.environ.get("EASY_RATHOLE_BUNDLES_DIR")
            os.environ["EASY_RATHOLE_RESOURCES_DIR"] = str(resources)
            os.environ["EASY_RATHOLE_BUNDLES_DIR"] = str(bundles)
            try:
                bundle_path = bundle_service.generate_windows7_bundle(
                    {
                        "public_ip": "10.10.10.10",
                        "rathole_control_port": 2333,
                    },
                    token="demo-token",
                )
            finally:
                if previous_resources is None:
                    os.environ.pop("EASY_RATHOLE_RESOURCES_DIR", None)
                else:
                    os.environ["EASY_RATHOLE_RESOURCES_DIR"] = previous_resources
                if previous_bundles is None:
                    os.environ.pop("EASY_RATHOLE_BUNDLES_DIR", None)
                else:
                    os.environ["EASY_RATHOLE_BUNDLES_DIR"] = previous_bundles

            self.assertTrue(bundle_path.exists())
            self.assertTrue(bundle_path.name.startswith("windows7-client-"))
            with zipfile.ZipFile(bundle_path) as zf:
                names = set(zf.namelist())
                self.assertIn(bundle_service.WINDOWS_SERVICE_WRAPPER_NAME, names)
                self.assertIn(bundle_service.WINDOWS_RATHOLE_BINARY_NAME, names)
                self.assertIn("client.toml", names)
                self.assertNotIn(bundle_service.WINDOWS_GUI_BINARY_NAME, names)


if __name__ == "__main__":
    unittest.main()
