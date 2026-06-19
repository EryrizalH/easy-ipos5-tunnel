from __future__ import annotations

import os
import sys

from .db import init_db


def main() -> int:
    db_path = os.environ.get("NUSA_TUNNEL_DB_PATH", "/opt/nusatunnel/state/nusatunnel.db")
    admin_username = os.environ.get("NUSA_TUNNEL_ADMIN_USERNAME", "admin")
    admin_password = os.environ.get("NUSA_TUNNEL_ADMIN_PASSWORD", "")
    token = os.environ.get("NUSA_TUNNEL_INITIAL_TOKEN", "")

    if not admin_password:
        raise SystemExit("NUSA_TUNNEL_ADMIN_PASSWORD is required for bootstrap")

    init_db(
        db_path=db_path,
        admin_username=admin_username,
        admin_password=admin_password,
        token=token,
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
