"""Tests for credential encryption."""

import json
import os

import pytest


def test_encrypt_decrypt_roundtrip():
    from app.credentials.encryption import decrypt_params, encrypt_params
    original = {"username": "admin", "password": "secret123"}
    key = os.urandom(32)
    encrypted = encrypt_params(original, key)
    assert isinstance(encrypted, bytes)
    assert encrypted != json.dumps(original).encode()
    decrypted = decrypt_params(encrypted, key)
    assert decrypted == original


def test_decrypt_with_wrong_key_fails():
    from app.credentials.encryption import decrypt_params, encrypt_params
    original = {"community": "public"}
    key1 = os.urandom(32)
    key2 = os.urandom(32)
    encrypted = encrypt_params(original, key1)
    with pytest.raises(ValueError, match="decrypt"):
        decrypt_params(encrypted, key2)


def test_encrypt_different_each_time():
    from app.credentials.encryption import encrypt_params
    data = {"username": "test"}
    key = os.urandom(32)
    enc1 = encrypt_params(data, key)
    enc2 = encrypt_params(data, key)
    assert enc1 != enc2
