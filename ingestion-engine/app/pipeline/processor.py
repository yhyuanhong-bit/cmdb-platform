"""Pipeline processor: orchestrates normalize, dedup, validate, and authority steps."""

import json
from uuid import UUID, uuid4

import asyncpg

from app.models.common import PipelineResult, RawAssetData
from app.pipeline.authority import apply_auto_merge, check_authority, create_conflicts
from app.pipeline.deduplicate import deduplicate
from app.pipeline.normalize import normalize
from app.pipeline.validate import validate_for_create, validate_for_update


async def process_single(
    pool: asyncpg.Pool, tenant_id: UUID, raw: RawAssetData
) -> PipelineResult:
    """Process a single raw asset through the full pipeline.

    Steps:
        1. Normalize field names and values
        2. Deduplicate (find existing asset)
        3. If new: validate for create, insert asset
        4. If existing: validate for update, authority check, auto-merge + conflicts

    Returns PipelineResult with action: created | updated | skipped | conflict
    """
    # 1. Normalize
    normalized = normalize(raw)

    # 2. Deduplicate
    dedup_result = await deduplicate(pool, tenant_id, normalized)

    if dedup_result.is_new:
        # 3. New asset: validate for create
        errors = validate_for_create(normalized)
        if errors:
            return PipelineResult(action="skipped", errors=errors)

        asset_id = await _create_asset(pool, tenant_id, normalized)
        return PipelineResult(asset_id=asset_id, action="created")
    else:
        # 4. Existing asset: validate for update
        errors = validate_for_update(normalized)
        if errors:
            return PipelineResult(
                asset_id=dedup_result.existing_asset_id,
                action="skipped",
                errors=errors,
            )

        # Authority check
        authority_result = await check_authority(
            pool,
            tenant_id,
            dedup_result.existing_asset_id,
            normalized.source,
            normalized.fields,
            dedup_result.existing_fields or {},
        )

        # Auto-merge authoritative fields
        await apply_auto_merge(
            pool,
            dedup_result.existing_asset_id,
            authority_result.auto_merge_fields,
        )

        # Create conflicts for non-authoritative changed fields
        conflict_ids = await create_conflicts(
            pool,
            tenant_id,
            dedup_result.existing_asset_id,
            normalized.source,
            authority_result.conflict_fields,
        )

        if conflict_ids:
            conflicts = [
                {
                    "id": str(cid),
                    "field": cf["field_name"],
                    "current": cf.get("current_value"),
                    "incoming": cf.get("incoming_value"),
                }
                for cid, cf in zip(conflict_ids, authority_result.conflict_fields)
            ]
            return PipelineResult(
                asset_id=dedup_result.existing_asset_id,
                action="conflict",
                conflicts=conflicts,
            )

        if authority_result.auto_merge_fields:
            return PipelineResult(
                asset_id=dedup_result.existing_asset_id,
                action="updated",
            )

        # No changes at all
        return PipelineResult(
            asset_id=dedup_result.existing_asset_id,
            action="skipped",
            errors=["No changes detected"],
        )


async def process_batch(
    pool: asyncpg.Pool,
    tenant_id: UUID,
    items: list[RawAssetData],
    progress_callback=None,
) -> dict:
    """Process a batch of raw assets through the pipeline.

    Args:
        pool: Database connection pool
        tenant_id: Tenant identifier
        items: List of raw asset data to process
        progress_callback: Optional callback called every 50 items with (processed, total)

    Returns:
        Dict with stats:
            {
                created, updated, skipped, conflicts,  # int counters
                error_count,                           # int, total failing rows
                errors,                                # list[dict] with per-row detail
            }

        Each element in ``errors`` is a structured record:
            {
                "row": <1-based row index>,
                "reason": <str(exc)>,
                "field": <offending field name if known, else None>,
                "exception_type": <exception class name>,
            }
    """
    stats: dict = {
        "created": 0,
        "updated": 0,
        "skipped": 0,
        "conflicts": 0,
        "error_count": 0,
        "errors": [],
    }

    for i, raw in enumerate(items):
        try:
            result = await process_single(pool, tenant_id, raw)

            if result.action == "created":
                stats["created"] += 1
            elif result.action == "updated":
                stats["updated"] += 1
            elif result.action == "conflict":
                stats["conflicts"] += 1
            elif result.action == "skipped":
                stats["skipped"] += 1
        except Exception as e:
            stats["error_count"] += 1
            stats["errors"].append(
                {
                    "row": i + 1,
                    "reason": str(e),
                    "field": getattr(e, "field", None),
                    "exception_type": type(e).__name__,
                }
            )

        # Progress callback every 50 items
        if progress_callback and (i + 1) % 50 == 0:
            progress_callback(i + 1, len(items))

    # Final progress callback
    if progress_callback and len(items) % 50 != 0:
        progress_callback(len(items), len(items))

    return stats


async def _create_asset(
    pool: asyncpg.Pool, tenant_id: UUID, raw: RawAssetData
) -> UUID:
    """Insert a new asset into the database.

    Uses fields from raw.fields for known columns and stores
    attributes as JSONB. Defaults status to 'inventoried' and
    bia_level to 'normal' if not provided.
    """
    asset_id = uuid4()

    asset_tag = raw.fields.get("asset_tag")
    name = raw.fields.get("name")
    asset_type = raw.fields.get("type")
    sub_type = raw.fields.get("sub_type")
    status = raw.fields.get("status", "inventoried")
    bia_level = raw.fields.get("bia_level", "normal")
    vendor = raw.fields.get("vendor")
    model = raw.fields.get("model")
    serial_number = raw.fields.get("serial_number")
    property_number = raw.fields.get("property_number")
    control_number = raw.fields.get("control_number")

    ip_address = raw.fields.get("ip_address")
    bmc_ip = raw.fields.get("bmc_ip")
    bmc_type = raw.fields.get("bmc_type")
    bmc_firmware = raw.fields.get("bmc_firmware")
    tags_str = raw.fields.get("tags")
    tags = [t.strip() for t in tags_str.split(",")] if tags_str else None

    # Warranty & lifecycle fields (all optional)
    purchase_date = raw.fields.get("purchase_date") or None
    purchase_cost_raw = raw.fields.get("purchase_cost")
    purchase_cost = float(purchase_cost_raw) if purchase_cost_raw else None
    warranty_start = raw.fields.get("warranty_start") or None
    warranty_end = raw.fields.get("warranty_end") or None
    warranty_vendor = raw.fields.get("warranty_vendor") or None
    warranty_contract = raw.fields.get("warranty_contract") or None
    lifespan_raw = raw.fields.get("expected_lifespan_months")
    expected_lifespan_months = int(lifespan_raw) if lifespan_raw else None
    eol_date = raw.fields.get("eol_date") or None

    # Resolve location name → location_id
    location_id = None
    location_name = raw.fields.get("location_name")
    if location_name:
        async with pool.acquire() as conn:
            row = await conn.fetchrow(
                "SELECT id FROM locations WHERE tenant_id = $1 AND (name = $2 OR name_en = $2 OR slug = $2) AND deleted_at IS NULL LIMIT 1",
                tenant_id,
                location_name.strip(),
            )
            if row:
                location_id = row["id"]

    # Resolve rack name → rack_id
    rack_id = None
    rack_name = raw.fields.get("rack_name")
    if rack_name:
        async with pool.acquire() as conn:
            row = await conn.fetchrow(
                "SELECT id FROM racks WHERE tenant_id = $1 AND name = $2 AND deleted_at IS NULL LIMIT 1",
                tenant_id,
                rack_name.strip(),
            )
            if row:
                rack_id = row["id"]

    attributes_json = json.dumps(raw.attributes) if raw.attributes else None

    async with pool.acquire() as conn:
        await conn.execute(
            """INSERT INTO assets (
                id, tenant_id, asset_tag, name, type, sub_type,
                status, bia_level, vendor, model, serial_number,
                property_number, control_number, attributes,
                ip_address, location_id, rack_id, tags,
                bmc_ip, bmc_type, bmc_firmware,
                purchase_date, purchase_cost, warranty_start, warranty_end,
                warranty_vendor, warranty_contract, expected_lifespan_months, eol_date
            ) VALUES (
                $1, $2, $3, $4, $5, $6,
                $7, $8, $9, $10, $11,
                $12, $13, $14,
                $15, $16, $17, $18,
                $19, $20, $21,
                $22, $23, $24, $25,
                $26, $27, $28, $29
            )""",
            asset_id,
            tenant_id,
            asset_tag,
            name,
            asset_type,
            sub_type,
            status,
            bia_level,
            vendor,
            model,
            serial_number,
            property_number,
            control_number,
            attributes_json,
            ip_address,
            location_id,
            rack_id,
            tags,
            bmc_ip,
            bmc_type,
            bmc_firmware,
            purchase_date,
            purchase_cost,
            warranty_start,
            warranty_end,
            warranty_vendor,
            warranty_contract,
            expected_lifespan_months,
            eol_date,
        )

    return asset_id
