"""Conflict resolution routes: list, resolve, batch-resolve."""

import json
import logging
import uuid
from datetime import datetime, timezone

import asyncpg
from fastapi import APIRouter, Depends, HTTPException

from app.dependencies import get_db_pool
from app.models.conflict import BatchConflictResolution, ConflictResolution

logger = logging.getLogger(__name__)

router = APIRouter(tags=["conflicts"])


@router.get("/conflicts")
async def list_conflicts(
    tenant_id: str,
    status: str = "pending",
    limit: int = 50,
    offset: int = 0,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """List conflicts for a tenant, filtered by status, paginated."""
    async with pool.acquire() as conn:
        rows = await conn.fetch(
            """SELECT * FROM conflicts
               WHERE tenant_id = $1 AND status = $2
               ORDER BY created_at DESC
               LIMIT $3 OFFSET $4""",
            uuid.UUID(tenant_id),
            status,
            limit,
            offset,
        )
        count = await conn.fetchval(
            "SELECT COUNT(*) FROM conflicts WHERE tenant_id = $1 AND status = $2",
            uuid.UUID(tenant_id),
            status,
        )
    return {
        "conflicts": [dict(r) for r in rows],
        "total": count,
        "limit": limit,
        "offset": offset,
    }


async def _resolve_single(
    pool: asyncpg.Pool,
    conflict_id: str,
    action: str,
    resolved_by: str,
) -> dict:
    """Resolve a single conflict: approve applies the value, reject discards it."""
    async with pool.acquire() as conn:
        row = await conn.fetchrow(
            "SELECT * FROM conflicts WHERE id = $1",
            uuid.UUID(conflict_id),
        )
        if not row:
            raise HTTPException(status_code=404, detail=f"Conflict {conflict_id} not found")
        if row["status"] != "pending":
            raise HTTPException(status_code=400, detail=f"Conflict {conflict_id} already resolved")

        if action == "approve":
            # Apply the incoming value to the asset
            await conn.execute(
                f"UPDATE assets SET {row['field_name']} = $1 WHERE id = $2",
                row["incoming_value"],
                row["asset_id"],
            )

        await conn.execute(
            """UPDATE conflicts
               SET status = $1, resolved_by = $2, resolved_at = $3
               WHERE id = $4""",
            "approved" if action == "approve" else "rejected",
            uuid.UUID(resolved_by),
            datetime.now(timezone.utc),
            uuid.UUID(conflict_id),
        )

    return {"conflict_id": conflict_id, "action": action, "status": "resolved"}


@router.post("/conflicts/{conflict_id}/resolve")
async def resolve_conflict(
    conflict_id: str,
    body: ConflictResolution,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Resolve a single conflict (approve or reject)."""
    return await _resolve_single(pool, conflict_id, body.action, str(body.resolved_by))


@router.post("/conflicts/batch-resolve")
async def batch_resolve_conflicts(
    body: BatchConflictResolution,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Resolve multiple conflicts at once."""
    results = []
    for cid in body.conflict_ids:
        try:
            result = await _resolve_single(pool, str(cid), body.action, str(body.resolved_by))
            results.append(result)
        except HTTPException as e:
            results.append({"conflict_id": str(cid), "error": e.detail})
    return {"results": results}
