from datetime import datetime
from uuid import UUID

from pydantic import BaseModel


class ImportConflict(BaseModel):
    id: UUID
    tenant_id: UUID
    asset_id: UUID
    source_type: str
    field_name: str
    current_value: str | None
    incoming_value: str | None
    status: str
    resolved_by: UUID | None = None
    resolved_at: datetime | None = None
    created_at: datetime


class ConflictResolution(BaseModel):
    action: str  # "approve" | "reject"
    resolved_by: UUID


class BatchConflictResolution(BaseModel):
    conflict_ids: list[UUID]
    action: str  # "approve" | "reject"
    resolved_by: UUID
