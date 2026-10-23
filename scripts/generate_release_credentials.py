"""Generate ignored Go credentials from environment, never command-line values."""
import json
import os
from pathlib import Path


def generate(root: Path) -> None:
    user = os.environ.get("IPOS5_DB_USERNAME", "")
    password = os.environ.get("IPOS5_DB_PASSWORD", "")
    if not user or not password:
        raise SystemExit("IPOS5_DB_USERNAME dan IPOS5_DB_PASSWORD wajib tersedia.")
    # UTF-8 literals avoid JSON surrogate escapes, which Go does not accept.
    quote = lambda value: json.dumps(value, ensure_ascii=False)
    path = root / "lock-ipos/internal/db/credentials_generated.go"
    path.write_text(
        "// Code generated for release. DO NOT COMMIT.\npackage db\n\n"
        f"func init() {{ User = {quote(user)}; Password = {quote(password)} }}\n",
        encoding="utf-8",
    )


if __name__ == "__main__":
    generate(Path(__file__).resolve().parents[1])
