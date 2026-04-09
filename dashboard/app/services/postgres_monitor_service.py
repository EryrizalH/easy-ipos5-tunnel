from __future__ import annotations

import os
import socket
import threading
import time
from datetime import UTC, datetime
from typing import Any

from ..db import connect, ensure_postgres_monitor_table, upsert_postgres_monitor_snapshot

try:
    import psycopg2  # type: ignore
except Exception:  # pragma: no cover
    psycopg2 = None


def utc_now() -> str:
    return datetime.now(UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def classify_status(query_ms: float | None, err: str) -> str:
    if err:
        return "Critical"
    if query_ms is None:
        return "Unknown"
    if query_ms < 100:
        return "Healthy"
    if query_ms <= 300:
        return "Warning"
    return "Critical"


def read_monitor_config() -> dict[str, Any]:
    enabled = os.environ.get("EASY_RATHOLE_PG_MONITOR_ENABLED", "1") == "1"
    interval_raw = os.environ.get("EASY_RATHOLE_PG_MONITOR_INTERVAL_SEC", "5").strip()
    try:
        interval_sec = max(1, int(interval_raw))
    except ValueError:
        interval_sec = 5

    dsn = os.environ.get("EASY_RATHOLE_PG_MONITOR_DSN", "").strip()
    host = os.environ.get("EASY_RATHOLE_PG_MONITOR_HOST", "127.0.0.1").strip() or "127.0.0.1"
    port = int(os.environ.get("EASY_RATHOLE_PG_MONITOR_PORT", "5444"))
    user = os.environ.get("EASY_RATHOLE_PG_MONITOR_USER", "sysi5adm").strip()
    password = os.environ.get("EASY_RATHOLE_PG_MONITOR_PASSWORD", "u&aV23cc.o82dtr1x89c").strip()
    dbname = os.environ.get("EASY_RATHOLE_PG_MONITOR_DBNAME", "postgres").strip()
    connect_timeout = max(1, int(os.environ.get("EASY_RATHOLE_PG_MONITOR_TIMEOUT_SEC", "3")))

    if not dsn:
        dsn = (
            f"host={host} port={port} dbname={dbname} user={user} "
            f"password={password} connect_timeout={connect_timeout}"
        )

    return {
        "enabled": enabled,
        "interval_sec": interval_sec,
        "dsn": dsn,
        "host": host,
        "port": port,
        "connect_timeout": connect_timeout,
    }


def _measure_tcp_connect_ms(host: str, port: int, timeout: int) -> float:
    started = time.perf_counter()
    with socket.create_connection((host, port), timeout=timeout):
        pass
    return (time.perf_counter() - started) * 1000


def _safe_int(value: Any) -> int | None:
    if value is None:
        return None
    try:
        return int(value)
    except (TypeError, ValueError):
        return None


def _safe_float(value: Any) -> float | None:
    if value is None:
        return None
    try:
        return float(value)
    except (TypeError, ValueError):
        return None


def run_postgres_probe(cfg: dict[str, Any]) -> dict[str, Any]:
    checked_at = utc_now()
    base: dict[str, Any] = {
        "client_id": "primary",
        "checked_at": checked_at,
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
        "last_error": "",
    }

    if psycopg2 is None:
        base["status"] = "Critical"
        base["last_error"] = "Dependency psycopg2 tidak tersedia."
        return base

    err_msg = ""
    conn = None
    try:
        base["connect_ms"] = round(_measure_tcp_connect_ms(cfg["host"], cfg["port"], cfg["connect_timeout"]), 2)
        conn = psycopg2.connect(cfg["dsn"])
        conn.autocommit = True
        with conn.cursor() as cur:
            started = time.perf_counter()
            cur.execute("SELECT 1;")
            cur.fetchone()
            base["query_ms"] = round((time.perf_counter() - started) * 1000, 2)

            started = time.perf_counter()
            cur.execute("BEGIN;")
            cur.execute("SELECT 1;")
            cur.fetchone()
            cur.execute("COMMIT;")
            base["tx_ms"] = round((time.perf_counter() - started) * 1000, 2)

            cur.execute(
                """
                SELECT
                  numbackends,
                  xact_commit,
                  xact_rollback,
                  blks_hit,
                  blks_read
                FROM pg_stat_database
                WHERE datname = current_database();
                """
            )
            db_row = cur.fetchone()

            cur.execute(
                """
                SELECT
                  COUNT(*) FILTER (WHERE state = 'active') AS active_connections,
                  COUNT(*) FILTER (WHERE waiting = true) AS waiting_connections
                FROM pg_stat_activity
                WHERE datname = current_database();
                """
            )
            activity_row = cur.fetchone()

            base["active_connections"] = _safe_int(activity_row[0] if activity_row else None)
            base["waiting_connections"] = _safe_int(activity_row[1] if activity_row else None)
            if db_row:
                base["xact_commit"] = _safe_int(db_row[1])
                base["xact_rollback"] = _safe_int(db_row[2])
                base["blks_hit"] = _safe_int(db_row[3])
                base["blks_read"] = _safe_int(db_row[4])
                blks_hit = _safe_float(db_row[3]) or 0.0
                blks_read = _safe_float(db_row[4]) or 0.0
                total = blks_hit + blks_read
                if total > 0:
                    base["cache_hit_ratio"] = round((blks_hit / total) * 100, 2)
    except Exception as exc:  # pragma: no cover
        err_msg = str(exc)
    finally:
        if conn is not None:
            conn.close()

    base["status"] = classify_status(base["query_ms"], err_msg)
    base["last_error"] = err_msg
    return base


class PostgresMonitorWorker:
    def __init__(self) -> None:
        self._stop_event = threading.Event()
        self._thread: threading.Thread | None = None
        self.cfg = read_monitor_config()

    def start(self) -> None:
        if not self.cfg["enabled"]:
            return
        if self._thread and self._thread.is_alive():
            return
        self._thread = threading.Thread(target=self._run_loop, name="pg-monitor-worker", daemon=True)
        self._thread.start()

    def stop(self) -> None:
        self._stop_event.set()
        if self._thread and self._thread.is_alive():
            self._thread.join(timeout=2)

    def _run_loop(self) -> None:
        while not self._stop_event.is_set():
            snap = run_postgres_probe(self.cfg)
            with connect() as conn:
                ensure_postgres_monitor_table(conn)
                upsert_postgres_monitor_snapshot(conn, snap)
            self._stop_event.wait(self.cfg["interval_sec"])
