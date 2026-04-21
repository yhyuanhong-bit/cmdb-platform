"""Tests for app.config deploy-mode + encryption-key guards."""

import importlib
import sys

import pytest


ENV_KEYS = (
    "INGESTION_DEPLOY_MODE",
    "INGESTION_CREDENTIAL_ENCRYPTION_KEY",
)


def _reload_config(monkeypatch):
    """Force a fresh import of app.config with the current env."""
    # Drop any cached copy so module-level validation runs again.
    for mod in ("app.config",):
        if mod in sys.modules:
            monkeypatch.delitem(sys.modules, mod, raising=False)
    return importlib.import_module("app.config")


def _clean_env(monkeypatch):
    for key in ENV_KEYS:
        monkeypatch.delenv(key, raising=False)


def test_deploy_mode_required(monkeypatch):
    """Unset deploy mode must fail loudly instead of silently defaulting."""
    _clean_env(monkeypatch)
    with pytest.raises((KeyError, RuntimeError)) as excinfo:
        _reload_config(monkeypatch)
    assert "INGESTION_DEPLOY_MODE" in str(excinfo.value)


def test_deploy_mode_invalid_rejected(monkeypatch):
    """Unknown deploy modes must be rejected with ValueError."""
    _clean_env(monkeypatch)
    monkeypatch.setenv("INGESTION_DEPLOY_MODE", "garbage")
    monkeypatch.setenv("INGESTION_CREDENTIAL_ENCRYPTION_KEY", "a" * 64)
    with pytest.raises(ValueError) as excinfo:
        _reload_config(monkeypatch)
    assert "INGESTION_DEPLOY_MODE" in str(excinfo.value)


def test_zero_key_rejected_in_production(monkeypatch):
    """The all-zero dev fallback key must never be accepted outside development."""
    _clean_env(monkeypatch)
    monkeypatch.setenv("INGESTION_DEPLOY_MODE", "production")
    monkeypatch.setenv("INGESTION_CREDENTIAL_ENCRYPTION_KEY", "0" * 64)
    with pytest.raises(ValueError) as excinfo:
        _reload_config(monkeypatch)
    assert "zero" in str(excinfo.value).lower()


def test_zero_key_rejected_in_staging(monkeypatch):
    """Staging is non-dev and must also reject the zero key."""
    _clean_env(monkeypatch)
    monkeypatch.setenv("INGESTION_DEPLOY_MODE", "staging")
    monkeypatch.setenv("INGESTION_CREDENTIAL_ENCRYPTION_KEY", "0" * 64)
    with pytest.raises(ValueError) as excinfo:
        _reload_config(monkeypatch)
    assert "zero" in str(excinfo.value).lower()


def test_zero_key_allowed_in_development(monkeypatch):
    """Development mode keeps the zero-key fallback for local convenience."""
    _clean_env(monkeypatch)
    monkeypatch.setenv("INGESTION_DEPLOY_MODE", "development")
    monkeypatch.setenv("INGESTION_CREDENTIAL_ENCRYPTION_KEY", "0" * 64)
    module = _reload_config(monkeypatch)
    assert module.settings.deploy_mode == "development"
    assert module.settings.credential_encryption_key == "0" * 64


def test_real_key_in_production_succeeds(monkeypatch):
    """A non-zero 64-hex key in production mode must import cleanly."""
    _clean_env(monkeypatch)
    prod_key = "a1b2c3d4" * 8  # 64 hex chars, non-zero
    monkeypatch.setenv("INGESTION_DEPLOY_MODE", "production")
    monkeypatch.setenv("INGESTION_CREDENTIAL_ENCRYPTION_KEY", prod_key)
    module = _reload_config(monkeypatch)
    assert module.settings.deploy_mode == "production"
    assert module.settings.credential_encryption_key == prod_key


def test_cmdb_core_url_default(monkeypatch):
    """Unset INGESTION_CMDB_CORE_URL must fall back to the localhost default.

    Multi-replica deployments override this per-replica via env, but the
    local dev loop still needs to work out of the box.
    """
    _clean_env(monkeypatch)
    monkeypatch.delenv("INGESTION_CMDB_CORE_URL", raising=False)
    monkeypatch.setenv("INGESTION_DEPLOY_MODE", "development")
    module = _reload_config(monkeypatch)
    assert module.settings.cmdb_core_url == "http://localhost:8080/api/v1"


def test_cmdb_core_url_env_override(monkeypatch):
    """Setting INGESTION_CMDB_CORE_URL must be honoured so multi-replica
    deployments can point at a shared LB or per-pod sidecar."""
    _clean_env(monkeypatch)
    monkeypatch.setenv("INGESTION_DEPLOY_MODE", "development")
    monkeypatch.setenv(
        "INGESTION_CMDB_CORE_URL", "http://cmdb-core.prod.svc:8080/api/v1"
    )
    module = _reload_config(monkeypatch)
    assert module.settings.cmdb_core_url == "http://cmdb-core.prod.svc:8080/api/v1"
