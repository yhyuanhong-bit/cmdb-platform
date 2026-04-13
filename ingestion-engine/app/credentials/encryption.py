"""AES-256-GCM encryption for credential params."""

import json
import os

from cryptography.hazmat.primitives.ciphers.aead import AESGCM


def encrypt_params(params: dict, key: bytes) -> bytes:
    """Encrypt a dict as JSON using AES-256-GCM. Returns nonce + ciphertext."""
    nonce = os.urandom(12)
    aesgcm = AESGCM(key)
    plaintext = json.dumps(params).encode("utf-8")
    ciphertext = aesgcm.encrypt(nonce, plaintext, None)
    return nonce + ciphertext


def decrypt_params(data: bytes, key: bytes) -> dict:
    """Decrypt AES-256-GCM encrypted params. Raises ValueError on failure."""
    if len(data) < 12:
        raise ValueError("Invalid encrypted data: too short")
    nonce = data[:12]
    ciphertext = data[12:]
    aesgcm = AESGCM(key)
    try:
        plaintext = aesgcm.decrypt(nonce, ciphertext, None)
    except Exception as e:
        raise ValueError(f"Failed to decrypt credential params: {e}") from e
    return json.loads(plaintext.decode("utf-8"))


def get_key_from_hex(hex_key: str) -> bytes:
    """Convert a 64-char hex string to 32 bytes."""
    return bytes.fromhex(hex_key)
