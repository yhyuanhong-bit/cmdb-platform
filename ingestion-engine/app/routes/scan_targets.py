"""Scan target management routes: list, create, update, delete."""

import logging
import uuid

import asyncpg
from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel

from app.dependencies import get_db_pool

logger = logging.getLogger(__name__)

router = APIRouter(tags=["scan-targets"])

VALID_COLLECTOR_TYPES = {"snmp", "ssh", "ipmi", "oneview", "dell_ome"}
VALID_MODES = {"auto", "review", "smart"}


class CreateScanTargetRequest(BaseModel):
    tenant_id: str
    name: str
    cidrs: list[str]
    collector_type: str
    credential_id: str
    mode: str = "smart"


class UpdateScanTargetRequest(BaseModel):
    name: str | None = None
    cidrs: list[str] | None = None
    collector_type: str | None = None
    credential_id: str | None = None
    mode: str | None = None


@router.get("/scan-targets")
async def list_scan_targets(
    tenant_id: str,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """List scan targets for a tenant with credential name via JOIN."""
    async with pool.acquire() as conn:
        rows = await conn.fetch(
            """SELECT st.*, c.name as credential_name
               FROM scan_targets st
               LEFT JOIN credentials c ON c.id = st.credential_id
               WHERE st.tenant_id = $1
               ORDER BY st.created_at DESC""",
            uuid.UUID(tenant_id),
        )
    return {"scan_targets": [dict(r) for r in rows]}


@router.post("/scan-targets", status_code=201)
async def create_scan_target(
    body: CreateScanTargetRequest,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Create a new scan target."""
    if body.collector_type not in VALID_COLLECTOR_TYPES:
        raise HTTPException(
            status_code=400,
            detail=f"collector_type must be one of: {', '.join(sorted(VALID_COLLECTOR_TYPES))}",
        )
    if body.mode not in VALID_MODES:
        raise HTTPException(
            status_code=400,
            detail=f"mode must be one of: {', '.join(sorted(VALID_MODES))}",
        )
    if not body.cidrs:
        raise HTTPException(status_code=400, detail="cidrs must not be empty")

    target_id = str(uuid.uuid4())

    async with pool.acquire() as conn:
        # Validate credential exists
        cred = await conn.fetchrow(
            "SELECT id FROM credentials WHERE id = $1",
            uuid.UUID(body.credential_id),
        )
        if not cred:
            raise HTTPException(status_code=404, detail="Credential not found")

        await conn.execute(
            """INSERT INTO scan_targets (id, tenant_id, name, cidrs, collector_type, credential_id, mode)
               VALUES ($1, $2, $3, $4, $5, $6, $7)""",
            uuid.UUID(target_id),
            uuid.UUID(body.tenant_id),
            body.name,
            body.cidrs,
            body.collector_type,
            uuid.UUID(body.credential_id),
            body.mode,
        )

        row = await conn.fetchrow(
            """SELECT st.*, c.name as credential_name
               FROM scan_targets st
               LEFT JOIN credentials c ON c.id = st.credential_id
               WHERE st.id = $1""",
            uuid.UUID(target_id),
        )

    return dict(row)


@router.put("/scan-targets/{target_id}")
async def update_scan_target(
    target_id: str,
    body: UpdateScanTargetRequest,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Update a scan target (dynamic SET clause)."""
    if body.collector_type is not None and body.collector_type not in VALID_COLLECTOR_TYPES:
        raise HTTPException(
            status_code=400,
            detail=f"collector_type must be one of: {', '.join(sorted(VALID_COLLECTOR_TYPES))}",
        )
    if body.mode is not None and body.mode not in VALID_MODES:
        raise HTTPException(
            status_code=400,
            detail=f"mode must be one of: {', '.join(sorted(VALID_MODES))}",
        )
    if body.cidrs is not None and not body.cidrs:
        raise HTTPException(status_code=400, detail="cidrs must not be empty")

    updates: dict = {}
    if body.name is not None:
        updates["name"] = body.name
    if body.cidrs is not None:
        updates["cidrs"] = body.cidrs
    if body.collector_type is not None:
        updates["collector_type"] = body.collector_type
    if body.credential_id is not None:
        updates["credential_id"] = uuid.UUID(body.credential_id)
    if body.mode is not None:
        updates["mode"] = body.mode

    if not updates:
        raise HTTPException(status_code=400, detail="No fields to update")

    async with pool.acquire() as conn:
        # Validate target exists
        existing = await conn.fetchrow(
            "SELECT id FROM scan_targets WHERE id = $1",
            uuid.UUID(target_id),
        )
        if not existing:
            raise HTTPException(status_code=404, detail="Scan target not found")

        # Validate credential if being updated
        if body.credential_id is not None:
            cred = await conn.fetchrow(
                "SELECT id FROM credentials WHERE id = $1",
                uuid.UUID(body.credential_id),
            )
            if not cred:
                raise HTTPException(status_code=404, detail="Credential not found")

        # Build dynamic SET clause
        set_parts = []
        values = []
        for i, (col, val) in enumerate(updates.items(), start=1):
            set_parts.append(f"{col} = ${i}")
            values.append(val)

        values.append(uuid.UUID(target_id))
        set_clause = ", ".join(set_parts)
        await conn.execute(
            f"UPDATE scan_targets SET {set_clause} WHERE id = ${len(values)}",
            *values,
        )

        row = await conn.fetchrow(
            """SELECT st.*, c.name as credential_name
               FROM scan_targets st
               LEFT JOIN credentials c ON c.id = st.credential_id
               WHERE st.id = $1""",
            uuid.UUID(target_id),
        )

    return dict(row)


@router.delete("/scan-targets/{target_id}", status_code=204)
async def delete_scan_target(
    target_id: str,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Delete a scan target."""
    async with pool.acquire() as conn:
        result = await conn.execute(
            "DELETE FROM scan_targets WHERE id = $1",
            uuid.UUID(target_id),
        )
    if result == "DELETE 0":
        raise HTTPException(status_code=404, detail="Scan target not found")
    return None
