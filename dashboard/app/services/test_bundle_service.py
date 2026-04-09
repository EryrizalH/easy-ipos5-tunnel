from __future__ import annotations

import json
import os
import tempfile
import unittest
import zipfile
from pathlib import Path

from app.services import bundle_service


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

            for name in (
                bundle_service.WINDOWS_BINARY_NAME,
                bundle_service.WINDOWS_GUI_BINARY_NAME,
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
                        "pgbouncer_databases": [
                            {"name": "iposdb"},
                            {"name": "masterdb", "backend_dbname": "backend_master"},
                        ],
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
                self.assertIn(bundle_service.WINDOWS_PGBOUNCER_DATABASES_NAME, names)
                payload = json.loads(zf.read(bundle_service.WINDOWS_PGBOUNCER_DATABASES_NAME).decode("utf-8"))

            self.assertEqual(
                payload,
                {
                    "databases": [
                        {"backend_dbname": "iposdb", "name": "iposdb"},
                        {"backend_dbname": "backend_master", "name": "masterdb"},
                    ]
                },
            )


if __name__ == "__main__":
    unittest.main()