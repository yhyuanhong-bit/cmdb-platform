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


def test_empty_key_raises_valueerror_in_production(monkeypatch):
    """W0.3: empty INGESTION_CREDENTIAL_ENCRYPTION_KEY in production must
    raise ValueError (previously called sys.exit(1), which is harder for
    callers to handle and bypasses logging frameworks)."""
    _clean_env(monkeypatch)
    monkeypatch.setenv("INGESTION_DEPLOY_MODE", "production")
    monkeypatch.setenv("INGESTION_CREDENTIAL_ENCRYPTION_KEY", "")
    with pytest.raises(ValueError) as excinfo:
        _reload_config(monkeypatch)
    assert "INGESTION_CREDENTIAL_ENCRYPTION_KEY" in str(excinfo.value)
    assert "production" in str(excinfo.value)


def test_empty_key_raises_valueerror_in_staging(monkeypatch):
    """W0.3: staging is non-dev so empty key must also hard-fail."""
    _clean_env(monkeypatch)
    monkeypatch.setenv("INGESTION_DEPLOY_MODE", "staging")
    monkeypatch.setenv("INGESTION_CREDENTIAL_ENCRYPTION_KEY", "")
    with pytest.raises(ValueError) as excinfo:
        _reload_config(monkeypatch)
    assert "INGESTION_CREDENTIAL_ENCRYPTION_KEY" in str(excinfo.value)


def test_short_key_rejected_in_production(monkeypatch):
    """W0.3: keys shorter than 32 chars must be rejected outside dev so a
    typo'd 8-char placeholder cannot reach a real environment."""
    _clean_env(monkeypatch)
    monkeypatch.setenv("INGESTION_DEPLOY_MODE", "production")
    monkeypatch.setenv("INGESTION_CREDENTIAL_ENCRYPTION_KEY", "abc123")
    with pytest.raises(ValueError) as excinfo:
        _reload_config(monkeypatch)
    msg = str(excinfo.value).lower()
    assert "too short" in msg or "32" in msg


def test_empty_key_in_dev_warns_and_falls_back(monkeypatch, caplog):
    """W0.3: development mode must keep the zero-key fallback (so local
    laptops Just Work) but must log a WARNING so the operator notices the
    degraded state."""
    _clean_env(monkeypatch)
    monkeypatch.setenv("INGESTION_DEPLOY_MODE", "development")
    monkeypatch.setenv("INGESTION_CREDENTIAL_ENCRYPTION_KEY", "")
    with caplog.at_level("WARNING", logger="app.config"):
        module = _reload_config(monkeypatch)
    # Fallback applied so encrypt() etc. don't blow up.
    assert module.settings.credential_encryption_key == "0" * 64
    # Operator-visible warning fired.
    warning_messages = [r.message for r in caplog.records if r.levelname == "WARNING"]
    assert any(
        "INGESTION_CREDENTIAL_ENCRYPTION_KEY" in m and "dev" in m.lower()
        for m in warning_messages
    ), f"expected dev-fallback warning, got: {warning_messages}"


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
