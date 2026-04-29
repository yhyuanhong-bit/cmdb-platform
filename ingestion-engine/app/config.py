import logging
import os

from pydantic_settings import BaseSettings

logger = logging.getLogger(__name__)

# Minimum acceptable length for the credential encryption key in any non-dev
# deploy mode. 32 characters is the floor; production setups should use a
# 64-char hex string (32 random bytes) generated via `secrets.token_hex(32)`.
_MIN_ENCRYPTION_KEY_LEN = 32


# Deploy mode must be set explicitly. A default here previously allowed
# production deployments to silently fall back to "development" — masking
# secret-management misconfiguration. Fail loudly instead.
try:
    _DEPLOY_MODE = os.environ["INGESTION_DEPLOY_MODE"]
except KeyError as exc:
    raise KeyError(
        "INGESTION_DEPLOY_MODE is not set. "
        "Set it to one of: development, staging, production."
    ) from exc

_ALLOWED_DEPLOY_MODES = ("development", "staging", "production")
if _DEPLOY_MODE not in _ALLOWED_DEPLOY_MODES:
    raise ValueError(
        f"invalid INGESTION_DEPLOY_MODE={_DEPLOY_MODE!r}; "
        f"must be one of {_ALLOWED_DEPLOY_MODES}"
    )


class Settings(BaseSettings):
    database_url: str = "postgresql://cmdb:cmdb@localhost:5432/cmdb"
    redis_url: str = "redis://localhost:6379/0"
    nats_url: str = "nats://localhost:4222"
    celery_broker_url: str = "redis://localhost:6379/1"
    upload_dir: str = "/tmp/cmdb-uploads"
    max_upload_size_mb: int = 50
    deploy_mode: str = _DEPLOY_MODE
    # tenant_id was previously a fallback default ("a0000000-...-001") which
    # silently routed every periodic scan and on-demand /mac-scan to the demo
    # tenant — audit E2/E3 (2026-04-28). All call sites now require tenant_id
    # from the caller and the periodic scan iterates every active tenant.
    # The field is removed; if any future code path still needs a global
    # default, add it back here with a hard fail-fast on the demo UUID.
    credential_encryption_key: str = ""
    # Service discovery: cmdb-core reachable address. Read from env so
    # multi-replica deployments can point each replica at its local
    # cmdb-core (or a shared LB) without rebuilding the image. Override
    # with INGESTION_CMDB_CORE_URL.
    cmdb_core_url: str = "http://localhost:8080/api/v1"

    model_config = {"env_prefix": "INGESTION_"}


settings = Settings()


def _is_zero_key(key: str) -> bool:
    """A key composed entirely of '0' chars is the dev fallback sentinel
    and offers no real encryption. Any length ≥1 made of only zeros counts."""
    return len(key) > 0 and set(key) == {"0"}


def _validate_encryption_key(deploy_mode: str, key: str) -> None:
    """Fail fast if the credential encryption key is unsafe for the current
    deploy mode. Non-dev environments must have a real key (non-empty,
    non-all-zeros, ≥32 chars). Dev mode is allowed to run with a degraded
    key for local convenience but logs a warning so the operator knows."""
    is_dev = deploy_mode == "development"

    if is_dev:
        if not key or _is_zero_key(key) or len(key) < _MIN_ENCRYPTION_KEY_LEN:
            logger.warning(
                "INGESTION_CREDENTIAL_ENCRYPTION_KEY is empty, all-zero, or "
                "shorter than %d chars; falling back to the all-zero dev key. "
                "Credentials are NOT meaningfully encrypted. This is only "
                "acceptable for local development (INGESTION_DEPLOY_MODE=development).",
                _MIN_ENCRYPTION_KEY_LEN,
            )
        return

    # Non-dev (staging / production): every weak-key shape must hard-fail.
    if not key:
        raise ValueError(
            "INGESTION_CREDENTIAL_ENCRYPTION_KEY is required in "
            f"deploy_mode={deploy_mode!r}; generate one with: "
            'python3 -c "import secrets; print(secrets.token_hex(32))"'
        )
    if _is_zero_key(key):
        raise ValueError(
            "zero encryption key only allowed in development mode; "
            "set INGESTION_CREDENTIAL_ENCRYPTION_KEY to a 64-character hex string "
            "(generate with: python3 -c \"import secrets; print(secrets.token_hex(32))\")"
        )
    if len(key) < _MIN_ENCRYPTION_KEY_LEN:
        raise ValueError(
            f"INGESTION_CREDENTIAL_ENCRYPTION_KEY is too short "
            f"({len(key)} chars); require at least {_MIN_ENCRYPTION_KEY_LEN} "
            f"chars in deploy_mode={deploy_mode!r}. Generate one with: "
            'python3 -c "import secrets; print(secrets.token_hex(32))"'
        )


_validate_encryption_key(settings.deploy_mode, settings.credential_encryption_key)

# Apply the dev fallback *after* validation so the warning above always
# reflects what the operator actually configured.
if settings.deploy_mode == "development" and (
    not settings.credential_encryption_key
    or len(settings.credential_encryption_key) < _MIN_ENCRYPTION_KEY_LEN
):
    settings.credential_encryption_key = "0" * 64
