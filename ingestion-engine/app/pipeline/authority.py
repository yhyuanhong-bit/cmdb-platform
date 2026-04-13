"""Authority check and conflict resolution for incoming asset data."""

from uuid import UUID

import asyncpg


class AuthorityResult:
    """Result of authority check for incoming fields."""

    def __init__(
        self,
        auto_merge_fields: dict | None = None,
        conflict_fields: list[dict] | None = None,
        skipped_fields: list[str] | None = None,
    ):
        self.auto_merge_fields = auto_merge_fields or {}
        self.conflict_fields = conflict_fields or []
        self.skipped_fields = skipped_fields or []


async def check_authority(
    pool: asyncpg.Pool,
    tenant_id: UUID,
    asset_id: UUID,
    source_type: str,
    incoming_fields: dict[str, str | None],
    existing_fields: dict[str, str | None],
) -> AuthorityResult:
    """Check field-level authority for incoming changes.

    For each changed field:
    - If source priority >= max priority for that field AND > 0: auto_merge
    - If values are the same: skip
    - Otherwise: conflict
    """
    authorities = await _load_authorities(pool, tenant_id)

    auto_merge: dict[str, str | None] = {}
    conflicts: list[dict] = []
    skipped: list[str] = []

    for field_name, incoming_value in incoming_fields.items():
        existing_value = existing_fields.get(field_name)

        # Same value - skip
        if str(incoming_value) == str(existing_value):
            skipped.append(field_name)
            continue

        source_priority = authorities.get((field_name, source_type), 0)
        max_priority = _get_max_priority(authorities, field_name)

        if source_priority >= max_priority and source_priority > 0:
            auto_merge[field_name] = incoming_value
        else:
            conflicts.append(
                {
                    "field_name": field_name,
                    "current_value": existing_value,
                    "incoming_value": incoming_value,
                }
            )

    return AuthorityResult(
        auto_merge_fields=auto_merge,
        conflict_fields=conflicts,
        skipped_fields=skipped,
    )


async def _load_authorities(
    pool: asyncpg.Pool, tenant_id: UUID
) -> dict[tuple[str, str], int]:
    """Load field authority mappings for a tenant."""
    async with pool.acquire() as conn:
        rows = await conn.fetch(
            "SELECT field_name, source_type, priority "
            "FROM asset_field_authorities WHERE tenant_id = $1",
            tenant_id,
        )
    return {(row["field_name"], row["source_type"]): row["priority"] for row in rows}


def _get_max_priority(
    authorities: dict[tuple[str, str], int], field_name: str
) -> int:
    """Get the maximum priority across all sources for a given field."""
    max_p = 0
    for (fname, _), priority in authorities.items():
        if fname == field_name and priority > max_p:
            max_p = priority
    return max_p


async def apply_auto_merge(
    pool: asyncpg.Pool, asset_id: UUID, fields_dict: dict[str, str | None]
) -> None:
    """Apply auto-merged fields to an asset."""
    if not fields_dict:
        return

    async with pool.acquire() as conn:
        for field_name, value in fields_dict.items():
            await conn.execute(
                f"UPDATE assets SET {field_name} = $1 WHERE id = $2",
                value,
                asset_id,
            )


async def create_conflicts(
    pool: asyncpg.Pool,
    tenant_id: UUID,
    asset_id: UUID,
    source_type: str,
    conflict_fields: list[dict],
) -> list[UUID]:
    """Create import_conflict records for fields that need manual resolution."""
    conflict_ids: list[UUID] = []

    async with pool.acquire() as conn:
        for conflict in conflict_fields:
            row = await conn.fetchrow(
                "INSERT INTO import_conflicts "
                "(tenant_id, asset_id, source_type, field_name, current_value, incoming_value) "
                "VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
                tenant_id,
                asset_id,
                source_type,
                conflict["field_name"],
                conflict.get("current_value"),
                conflict.get("incoming_value"),
            )
            conflict_ids.append(row["id"])

    return conflict_ids
