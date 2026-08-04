from __future__ import annotations

import os
import tempfile
import unittest
import zipfile
from pathlib import Path

from app.services import bundle_service


def write_self_extracting_installer(path: Path, *, includes_gui: bool) -> None:
    path.write_bytes(b"MZstub")
    with zipfile.ZipFile(path, "a") as archive:
        for name in bundle_service.WINDOWS_EMBEDDED_RUNTIME_NAMES:
            archive.writestr(name, b"x")
        if includes_gui:
            archive.writestr(bundle_service.WINDOWS_GUI_BINARY_NAME, b"x")


class BundleServiceTest(unittest.TestCase):
    def test_normalize_pgbouncer_databases_defaults(self) -> None:
        rows = bundle_service.normalize_pgbouncer_databases(None)
        self.assertEqual(rows, [{"name": "postgres", "backend_dbname": "postgres"}])

    def test_normalize_pgbouncer_databases_dedupes_and_trims(self) -> None:
        rows = bundle_service.normalize_pgbouncer_databases(
            [
                "iposdb",
                {"name": " masterdb ", "backend_dbname": " backend_a "},
                {"name": "iposdb"},
                {"name": ""},
            ]
        )
        self.assertEqual(
            rows,
            [
                {"name": "iposdb", "backend_dbname": "iposdb"},
                {"name": "masterdb", "backend_dbname": "backend_a"},
            ],
        )

    def test_generate_windows_bundle_includes_database_metadata(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            resources = root / "resources"
            bundles = root / "bundles"
            assets = resources / "assets" / "windows"
            templates = resources / "templates" / "rathole"
            assets.mkdir(parents=True, exist_ok=True)
            templates.mkdir(parents=True, exist_ok=True)
            bundles.mkdir(parents=True, exist_ok=True)

            write_self_extracting_installer(assets / bundle_service.WINDOWS_UNIFIED_NAME, includes_gui=True)
            (templates / "client.toml.tpl").write_text(
                "remote_addr={{SERVER_ADDR}}:{{RATHOLE_CONTROL_PORT}}\n"
                "db={{DB_SERVICE_KEY}} {{DB_CLIENT_LOCAL_ADDR}}\n",
                encoding="utf-8",
            )

            previous_resources = os.environ.get("NUSA_TUNNEL_RESOURCES_DIR")
            previous_bundles = os.environ.get("NUSA_TUNNEL_BUNDLES_DIR")
            os.environ["NUSA_TUNNEL_RESOURCES_DIR"] = str(resources)
            os.environ["NUSA_TUNNEL_BUNDLES_DIR"] = str(bundles)
            try:
                bundle_path = bundle_service.generate_windows_bundle(
                    {
                        "public_ip": "10.10.10.10",
                        "pgbouncer_databases": [
                            {"name": "iposdb"},
                            {"name": "masterdb", "backend_dbname": "backend_master"},
                        ],
                    },
                    token="demo-token",
                )
            finally:
                if previous_resources is None:
                    os.environ.pop("NUSA_TUNNEL_RESOURCES_DIR", None)
                else:
                    os.environ["NUSA_TUNNEL_RESOURCES_DIR"] = previous_resources
                if previous_bundles is None:
                    os.environ.pop("NUSA_TUNNEL_BUNDLES_DIR", None)
                else:
                    os.environ["NUSA_TUNNEL_BUNDLES_DIR"] = previous_bundles

            self.assertTrue(bundle_path.exists())
            with zipfile.ZipFile(bundle_path) as zf:
                names = set(zf.namelist())
                client_toml = zf.read("client.toml").decode("utf-8")

            self.assertEqual(names, {bundle_service.WINDOWS_UNIFIED_NAME, "client.toml"})
            self.assertIn("127.0.0.1:5444", client_toml)
            self.assertIn("remote_addr=10.10.10.10:443", client_toml)
            self.assertIn('"backend_dbname":"backend_master"', client_toml)

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

            write_self_extracting_installer(assets / bundle_service.WINDOWS_UNIFIED_NAME, includes_gui=False)
            (templates / "client.toml.tpl").write_text(
                "remote_addr={{SERVER_ADDR}}:{{RATHOLE_CONTROL_PORT}}\n"
                "db={{DB_SERVICE_KEY}} {{DB_CLIENT_LOCAL_ADDR}}\n",
                encoding="utf-8",
            )

            previous_resources = os.environ.get("NUSA_TUNNEL_RESOURCES_DIR")
            previous_bundles = os.environ.get("NUSA_TUNNEL_BUNDLES_DIR")
            os.environ["NUSA_TUNNEL_RESOURCES_DIR"] = str(resources)
            os.environ["NUSA_TUNNEL_BUNDLES_DIR"] = str(bundles)
            try:
                bundle_path = bundle_service.generate_windows7_bundle(
                    {
                        "public_ip": "10.10.10.10",
                        "rathole_control_port": 443,
                    },
                    token="demo-token",
                )
            finally:
                if previous_resources is None:
                    os.environ.pop("NUSA_TUNNEL_RESOURCES_DIR", None)
                else:
                    os.environ["NUSA_TUNNEL_RESOURCES_DIR"] = previous_resources
                if previous_bundles is None:
                    os.environ.pop("NUSA_TUNNEL_BUNDLES_DIR", None)
                else:
                    os.environ["NUSA_TUNNEL_BUNDLES_DIR"] = previous_bundles

            self.assertTrue(bundle_path.exists())
            self.assertTrue(bundle_path.name.startswith("windows7-client-"))
            with zipfile.ZipFile(bundle_path) as zf:
                names = set(zf.namelist())
                self.assertEqual(names, {bundle_service.WINDOWS_UNIFIED_NAME, "client.toml"})

    def test_windows_bundle_rejects_installer_without_payload(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            resources = root / "resources"
            assets = resources / "assets" / "windows"
            templates = resources / "templates" / "rathole"
            assets.mkdir(parents=True, exist_ok=True)
            templates.mkdir(parents=True, exist_ok=True)
            (assets / bundle_service.WINDOWS_UNIFIED_NAME).write_bytes(b"MZstub")
            (templates / "client.toml.tpl").write_text("remote_addr={{SERVER_ADDR}}:{{RATHOLE_CONTROL_PORT}}\n", encoding="utf-8")

            previous_resources = os.environ.get("NUSA_TUNNEL_RESOURCES_DIR")
            os.environ["NUSA_TUNNEL_RESOURCES_DIR"] = str(resources)
            try:
                with self.assertRaises(RuntimeError):
                    bundle_service.generate_windows_bundle({"public_ip": "10.10.10.10"}, token="demo-token")
            finally:
                if previous_resources is None:
                    os.environ.pop("NUSA_TUNNEL_RESOURCES_DIR", None)
                else:
                    os.environ["NUSA_TUNNEL_RESOURCES_DIR"] = previous_resources

    def test_build_windows_installer_payload_from_raw_setup(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            assets = root / "assets"
            source = root / "setup-raw.exe"
            target = root / "output" / bundle_service.WINDOWS_UNIFIED_NAME
            assets.mkdir()
            source.write_bytes(b"MZstub")
            for name in (*bundle_service.WINDOWS_EMBEDDED_RUNTIME_NAMES, bundle_service.WINDOWS_GUI_BINARY_NAME):
                (assets / name).write_bytes(b"runtime")

            bundle_service.build_windows_installer_payload(source, assets, target, includes_gui=True)

            self.assertTrue(target.read_bytes().startswith(b"MZstub"))
            bundle_service.require_windows_installer_payload(target, includes_gui=True)

    def test_generate_linux_bundle_contains_two_files(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            resources = root / "resources"
            bundles = root / "bundles"
            templates = resources / "templates" / "rathole"
            linux_assets = resources / "assets" / "linux"
            templates.mkdir(parents=True, exist_ok=True)
            linux_assets.mkdir(parents=True, exist_ok=True)
            bundles.mkdir(parents=True, exist_ok=True)
            (templates / "client.toml.tpl").write_text("remote_addr={{SERVER_ADDR}}:{{RATHOLE_CONTROL_PORT}}\n", encoding="utf-8")
            (linux_assets / "install-client.sh.tpl").write_text("#!/usr/bin/env bash\n", encoding="utf-8")

            previous_resources = os.environ.get("NUSA_TUNNEL_RESOURCES_DIR")
            previous_bundles = os.environ.get("NUSA_TUNNEL_BUNDLES_DIR")
            os.environ["NUSA_TUNNEL_RESOURCES_DIR"] = str(resources)
            os.environ["NUSA_TUNNEL_BUNDLES_DIR"] = str(bundles)
            try:
                bundle_path = bundle_service.generate_linux_bundle({"public_ip": "10.10.10.10"}, token="demo-token")
            finally:
                if previous_resources is None:
                    os.environ.pop("NUSA_TUNNEL_RESOURCES_DIR", None)
                else:
                    os.environ["NUSA_TUNNEL_RESOURCES_DIR"] = previous_resources
                if previous_bundles is None:
                    os.environ.pop("NUSA_TUNNEL_BUNDLES_DIR", None)
                else:
                    os.environ["NUSA_TUNNEL_BUNDLES_DIR"] = previous_bundles

            with zipfile.ZipFile(bundle_path) as archive:
                self.assertEqual(set(archive.namelist()), {"client.toml", "install-client.sh"})


if __name__ == "__main__":
    unittest.main()
