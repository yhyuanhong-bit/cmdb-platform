from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    database_url: str = "postgresql://cmdb:cmdb@localhost:5432/cmdb"
    redis_url: str = "redis://localhost:6379/0"
    nats_url: str = "nats://localhost:4222"
    celery_broker_url: str = "redis://localhost:6379/1"
    upload_dir: str = "/tmp/cmdb-uploads"
    max_upload_size_mb: int = 50
    deploy_mode: str = "development"
    tenant_id: str = "a0000000-0000-0000-0000-000000000001"
    credential_encryption_key: str = ""

    model_config = {"env_prefix": "INGESTION_"}


import sys

settings = Settings()

if not settings.credential_encryption_key or settings.credential_encryption_key == "0" * 64:
    if settings.deploy_mode != "development":
        print("FATAL: INGESTION_CREDENTIAL_ENCRYPTION_KEY must be set to a 64-character hex string", file=sys.stderr)
        print("Generate one with: python3 -c \"import secrets; print(secrets.token_hex(32))\"", file=sys.stderr)
        sys.exit(1)
    else:
        # Development mode fallback
        settings.credential_encryption_key = "0" * 64
