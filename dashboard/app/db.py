from __future__ import annotations

import os
import sqlite3
from datetime import UTC, datetime
from pathlib import Path

from .auth import hash_password

DEFAULT_DB_PATH = "/opt/easy-rathole/state/easy-rathole.db"


def utc_now() -> str:
    return datetime.now(UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def get_db_path() -> str:
    return os.environ.get("EASY_RATHOLE_DB_PATH", DEFAULT_DB_PATH)


def connect(db_path: str | None = None) -> sqlite3.Connection:
    path = Path(db_path or get_db_path())
    path.parent.mkdir(parents=True, exist_ok=True)
    conn = sqlite3.connect(path)
    conn.row_factory = sqlite3.Row
    return conn


def init_db(db_path: str, admin_username: str, admin_password: str, token: str) -> None:
    now = utc_now()
    with connect(db_path) as conn:
        conn.executescript(
            """
            CREATE TABLE IF NOT EXISTS users (
              username TEXT PRIMARY KEY,
              password_hash TEXT NOT NULL,
              salt TEXT NOT NULL,
              created_at TEXT NOT NULL,
              updated_at TEXT NOT NULL
            );

            CREATE TABLE IF NOT EXISTS settings (
              key TEXT PRIMARY KEY,
              value TEXT NOT NULL,
              updated_at TEXT NOT NULL
            );
            """
        )

        salt_hex, hash_hex = hash_password(admin_password)
        conn.execute(
            """
            INSERT INTO users (username, password_hash, salt, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?)
            ON CONFLICT(username) DO UPDATE SET
              password_hash=excluded.password_hash,
              salt=excluded.salt,
              updated_at=excluded.updated_at
            """,
            (admin_username, hash_hex, salt_hex, now, now),
        )

        if token:
            set_setting(conn, "global_token", token)
        set_setting(conn, "last_bootstrap", now)
        conn.commit()


def get_user(conn: sqlite3.Connection, username: str) -> sqlite3.Row | None:
    return conn.execute(
        "SELECT username, password_hash, salt FROM users WHERE username = ?",
        (username,),
    ).fetchone()


def get_setting(conn: sqlite3.Connection, key: str, default: str = "") -> str:
    row = conn.execute("SELECT value FROM settings WHERE key = ?", (key,)).fetchone()
    if not row:
        return default
    return str(row["value"])


def set_setting(conn: sqlite3.Connection, key: str, value: str) -> None:
    now = utc_now()
    conn.execute(
        """
        INSERT INTO settings (key, value, updated_at)
        VALUES (?, ?, ?)
        ON CONFLICT(key) DO UPDATE SET
          value=excluded.value,
          updated_at=excluded.updated_at
        """,
        (key, value, now),
    )
