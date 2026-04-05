from __future__ import annotations

import json
import os
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

DEFAULT_STATE_PATH = "/opt/easy-rathole/state/install-state.json"


def utc_now() -> str:
    return datetime.now(UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def get_state_path() -> Path:
    return Path(os.environ.get("EASY_RATHOLE_STATE_FILE", DEFAULT_STATE_PATH))


def load_state(path: Path | None = None) -> dict[str, Any]:
    state_path = path or get_state_path()
    if not state_path.exists():
        return {}
    return json.loads(state_path.read_text(encoding="utf-8"))


def save_state(data: dict[str, Any], path: Path | None = None) -> None:
    state_path = path or get_state_path()
    state_path.parent.mkdir(parents=True, exist_ok=True)
    state_path.write_text(json.dumps(data, indent=2, sort_keys=True), encoding="utf-8")


def merge_state(patch: dict[str, Any], path: Path | None = None) -> dict[str, Any]:
    data = load_state(path)
    data.update(patch)
    data["updated_at"] = utc_now()
    save_state(data, path)
    return data
