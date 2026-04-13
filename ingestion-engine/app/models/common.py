from datetime import datetime
from uuid import UUID

from pydantic import BaseModel


class RawAssetData(BaseModel):
    source: str
    unique_key: str
    fields: dict[str, str | None]
    attributes: dict | None = None
    collected_at: datetime | None = None


class FieldMapping(BaseModel):
    field_name: str
    authority: bool = False


class CollectTarget(BaseModel):
    endpoint: str
    credentials: dict | None = None
    options: dict | None = None


class ConnectionResult(BaseModel):
    success: bool
    message: str = ""
    latency_ms: float | None = None


class PipelineResult(BaseModel):
    asset_id: UUID | None = None
    action: str
    conflicts: list[dict] | None = None
    errors: list[str] | None = None
