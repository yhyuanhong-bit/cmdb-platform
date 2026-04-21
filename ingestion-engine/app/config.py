import os
import sys

from pydantic_settings import BaseSettings


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
    tenant_id: str = "a0000000-0000-0000-0000-000000000001"
    credential_encryption_key: str = ""
    # Service discovery: cmdb-core reachable address. Read from env so
    # multi-replica deployments can point each replica at its local
    # cmdb-core (or a shared LB) without rebuilding the image. Override
    # with INGESTION_CMDB_CORE_URL.
    cmdb_core_url: str = "http://localhost:8080/api/v1"

    model_config = {"env_prefix": "INGESTION_"}


settings = Settings()

# Guard against the zero-key dev fallback leaking into any non-development
# environment. Previously the "== 0*64" path silently rewrote the key in dev
# but the check to *reject* it lived behind `deploy_mode != "development"`,
# which defaulted to development — so a forgotten env var could keep the
# zero key in prod. Deploy mode is now required above, and we reject the
# zero key for every non-dev mode.
if settings.deploy_mode != "development" and settings.credential_encryption_key == "0" * 64:
    raise ValueError(
        "zero encryption key only allowed in development mode; "
        "set INGESTION_CREDENTIAL_ENCRYPTION_KEY to a 64-character hex string "
        "(generate with: python3 -c \"import secrets; print(secrets.token_hex(32))\")"
    )

if not settings.credential_encryption_key:
    if settings.deploy_mode != "development":
        print(
            "FATAL: INGESTION_CREDENTIAL_ENCRYPTION_KEY must be set to a 64-character hex string",
            file=sys.stderr,
        )
        print(
            "Generate one with: python3 -c \"import secrets; print(secrets.token_hex(32))\"",
            file=sys.stderr,
        )
        sys.exit(1)
    # Development mode fallback
    settings.credential_encryption_key = "0" * 64
