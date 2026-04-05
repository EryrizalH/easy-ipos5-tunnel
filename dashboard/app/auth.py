from __future__ import annotations

import hashlib
import hmac
import os

PBKDF2_ITERATIONS = 200_000


def hash_password(password: str, salt_hex: str | None = None) -> tuple[str, str]:
    if not password:
        raise ValueError("Password must not be empty")

    salt = bytes.fromhex(salt_hex) if salt_hex else os.urandom(16)
    digest = hashlib.pbkdf2_hmac(
        "sha256",
        password.encode("utf-8"),
        salt,
        PBKDF2_ITERATIONS,
    )
    return salt.hex(), digest.hex()


def verify_password(password: str, salt_hex: str, expected_hash_hex: str) -> bool:
    _, hash_hex = hash_password(password, salt_hex=salt_hex)
    return hmac.compare_digest(hash_hex, expected_hash_hex)
