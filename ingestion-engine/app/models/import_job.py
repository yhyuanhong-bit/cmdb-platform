from datetime import datetime
from uuid import UUID

from pydantic import BaseModel


class ImportJobCreate(BaseModel):
    tenant_id: UUID
    type: str
    filename: str
    uploaded_by: UUID | None = None


class ImportJob(BaseModel):
    id: UUID
    tenant_id: UUID
    type: str
    filename: str
    status: str
    total_rows: int
    processed_rows: int
    stats: dict
    error_details: list[dict]
    uploaded_by: UUID | None = None
    created_at: datetime
    completed_at: datetime | None = None


class ParsedRow(BaseModel):
    row_num: int
    data: dict[str, str | None]
    errors: list[str] | None = None


class ParseResult(BaseModel):
    total_rows: int
    valid_rows: list[ParsedRow]
    error_rows: list[ParsedRow]
    preview: list[ParsedRow]
