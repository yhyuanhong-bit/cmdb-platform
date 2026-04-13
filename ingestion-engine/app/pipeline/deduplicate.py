"""Deduplicate incoming asset data against existing assets in the database."""

from uuid import UUID

import asyncpg

from app.models.common import RawAssetData


class DeduplicateResult:
    """Result of deduplication lookup."""

    def __init__(
        self,
        existing_asset_id: UUID | None = None,
        existing_fields: dict | None = None,
    ):
        self.existing_asset_id = existing_asset_id
        self.existing_fields = existing_fields

    @property
    def is_new(self) -> bool:
        return self.existing_asset_id is None


async def deduplicate(
    pool: asyncpg.Pool, tenant_id: UUID, raw: RawAssetData
) -> DeduplicateResult:
    """Try to match incoming data to an existing asset.

    Match order: serial_number first, then asset_tag.
    """
    async with pool.acquire() as conn:
        # Try serial_number match first
        serial = raw.fields.get("serial_number")
        if serial:
            row = await conn.fetchrow(
                "SELECT id, asset_tag, name, type, sub_type, status, bia_level, "
                "vendor, model, serial_number, property_number, control_number "
                "FROM assets WHERE tenant_id = $1 AND serial_number = $2",
                tenant_id,
                serial,
            )
            if row:
                return _row_to_result(row)

        # Try asset_tag match
        asset_tag = raw.fields.get("asset_tag")
        if asset_tag:
            row = await conn.fetchrow(
                "SELECT id, asset_tag, name, type, sub_type, status, bia_level, "
                "vendor, model, serial_number, property_number, control_number "
                "FROM assets WHERE tenant_id = $1 AND asset_tag = $2",
                tenant_id,
                asset_tag,
            )
            if row:
                return _row_to_result(row)

    return DeduplicateResult()


def _row_to_result(row: asyncpg.Record) -> DeduplicateResult:
    """Convert a database row to a DeduplicateResult."""
    fields = {
        k: str(v) if v is not None else None
        for k, v in dict(row).items()
        if k != "id"
    }
    return DeduplicateResult(
        existing_asset_id=row["id"],
        existing_fields=fields,
    )
