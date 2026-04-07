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
    credential_encryption_key: str = "0" * 64

    model_config = {"env_prefix": "INGESTION_"}


settings = Settings()
