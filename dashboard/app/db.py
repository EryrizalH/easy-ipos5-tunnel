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

            CREATE TABLE IF NOT EXISTS postgres_monitor_latest (
              client_id TEXT PRIMARY KEY,
              checked_at TEXT NOT NULL,
              status TEXT NOT NULL,
              connect_ms REAL,
              query_ms REAL,
              tx_ms REAL,
              active_connections INTEGER,
              waiting_connections INTEGER,
              xact_commit BIGINT,
              xact_rollback BIGINT,
              blks_hit BIGINT,
              blks_read BIGINT,
              cache_hit_ratio REAL,
              last_error TEXT
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


def ensure_postgres_monitor_table(conn: sqlite3.Connection) -> None:
    conn.execute(
        """
        CREATE TABLE IF NOT EXISTS postgres_monitor_latest (
          client_id TEXT PRIMARY KEY,
          checked_at TEXT NOT NULL,
          status TEXT NOT NULL,
          connect_ms REAL,
          query_ms REAL,
          tx_ms REAL,
          active_connections INTEGER,
          waiting_connections INTEGER,
          xact_commit BIGINT,
          xact_rollback BIGINT,
          blks_hit BIGINT,
          blks_read BIGINT,
          cache_hit_ratio REAL,
          last_error TEXT
        )
        """
    )
    conn.commit()


def upsert_postgres_monitor_snapshot(conn: sqlite3.Connection, snapshot: dict[str, object]) -> None:
    conn.execute(
        """
        INSERT INTO postgres_monitor_latest (
          client_id, checked_at, status, connect_ms, query_ms, tx_ms,
          active_connections, waiting_connections, xact_commit, xact_rollback,
          blks_hit, blks_read, cache_hit_ratio, last_error
        )
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(client_id) DO UPDATE SET
          checked_at=excluded.checked_at,
          status=excluded.status,
          connect_ms=excluded.connect_ms,
          query_ms=excluded.query_ms,
          tx_ms=excluded.tx_ms,
          active_connections=excluded.active_connections,
          waiting_connections=excluded.waiting_connections,
          xact_commit=excluded.xact_commit,
          xact_rollback=excluded.xact_rollback,
          blks_hit=excluded.blks_hit,
          blks_read=excluded.blks_read,
          cache_hit_ratio=excluded.cache_hit_ratio,
          last_error=excluded.last_error
        """,
        (
            snapshot.get("client_id", "primary"),
            snapshot.get("checked_at", utc_now()),
            snapshot.get("status", "Critical"),
            snapshot.get("connect_ms"),
            snapshot.get("query_ms"),
            snapshot.get("tx_ms"),
            snapshot.get("active_connections"),
            snapshot.get("waiting_connections"),
            snapshot.get("xact_commit"),
            snapshot.get("xact_rollback"),
            snapshot.get("blks_hit"),
            snapshot.get("blks_read"),
            snapshot.get("cache_hit_ratio"),
            snapshot.get("last_error", ""),
        ),
    )
    conn.commit()


def get_postgres_monitor_latest(conn: sqlite3.Connection, client_id: str = "primary") -> dict[str, object]:
    row = conn.execute(
        """
        SELECT
          client_id, checked_at, status, connect_ms, query_ms, tx_ms,
          active_connections, waiting_connections, xact_commit, xact_rollback,
          blks_hit, blks_read, cache_hit_ratio, last_error
        FROM postgres_monitor_latest
        WHERE client_id = ?
        """,
        (client_id,),
    ).fetchone()
    if not row:
        return {
            "client_id": client_id,
            "checked_at": "-",
            "status": "Unknown",
            "connect_ms": None,
            "query_ms": None,
            "tx_ms": None,
            "active_connections": None,
            "waiting_connections": None,
            "xact_commit": None,
            "xact_rollback": None,
            "blks_hit": None,
            "blks_read": None,
            "cache_hit_ratio": None,
            "last_error": "Belum ada data probe PostgreSQL.",
        }
    return dict(row)


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
