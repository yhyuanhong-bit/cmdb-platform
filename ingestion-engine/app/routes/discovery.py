"""Discovery scan and task management routes."""

import json
import logging
import uuid
from typing import Optional

import asyncpg
from fastapi import APIRouter, Depends, HTTPException, Query
from pydantic import BaseModel

from nats.aio.client import Client as NATSClient

from app.dependencies import get_db_pool, get_nats
from app.tasks.discovery_task import process_discovery_task
from app.tasks.mac_scan_task import run_mac_scan

logger = logging.getLogger(__name__)

router = APIRouter(tags=["discovery"])


class ScanRequest(BaseModel):
    scan_target_id: Optional[str] = None
    tenant_id: Optional[str] = None
    collector_type: Optional[str] = None
    cidrs: Optional[list[str]] = None
    credential_id: Optional[str] = None
    mode: Optional[str] = "smart"


@router.post("/discovery/scan", status_code=201)
async def trigger_scan(
    body: ScanRequest,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Trigger a discovery scan from a scan_target_id or inline config."""
    async with pool.acquire() as conn:
        if body.scan_target_id is not None:
            row = await conn.fetchrow(
                "SELECT tenant_id, collector_type, cidrs, credential_id, mode "
                "FROM scan_targets WHERE id = $1",
                uuid.UUID(body.scan_target_id),
            )
            if not row:
                raise HTTPException(status_code=404, detail="Scan target not found")
            tenant_id = str(row["tenant_id"])
            collector_type = row["collector_type"]
            cidrs = list(row["cidrs"])
            credential_id = str(row["credential_id"])
            mode = row["mode"]
        else:
            missing = [
                f for f in ("tenant_id", "collector_type", "cidrs", "credential_id")
                if getattr(body, f) is None
            ]
            if missing:
                raise HTTPException(
                    status_code=400,
                    detail=f"Missing required fields: {', '.join(missing)}",
                )
            tenant_id = body.tenant_id
            collector_type = body.collector_type
            cidrs = body.cidrs
            credential_id = body.credential_id
            # Wave 3: review-gate default. Pre-3 the default was "smart",
            # which auto-merged any discovery that matched an existing CI
            # without operator review — a scanner misconfiguration could
            # silently poison the CMDB. Default is now "review", which
            # lands everything in the staging queue; callers who want
            # the old behaviour must opt in by explicitly setting
            # mode="auto" or mode="smart".
            mode = body.mode or "review"

        task_id = uuid.uuid4()
        config = {
            "collector_type": collector_type,
            "cidrs": cidrs,
            "credential_id": credential_id,
            "mode": mode,
        }
        if body.scan_target_id:
            config["scan_target_id"] = body.scan_target_id

        await conn.execute(
            """INSERT INTO discovery_tasks (id, tenant_id, type, status, config)
               VALUES ($1, $2, $3, $4, $5)""",
            task_id,
            uuid.UUID(tenant_id),
            collector_type,
            "pending",
            json.dumps(config),
        )

    process_discovery_task.delay(
        str(task_id),
        tenant_id,
        collector_type,
        cidrs,
        credential_id,
        mode,
    )

    return {"task_id": str(task_id), "status": "pending", "type": collector_type}


@router.get("/discovery/tasks")
async def list_tasks(
    tenant_id: str,
    status: Optional[str] = Query(default=None),
    limit: int = Query(default=50, ge=1, le=500),
    offset: int = Query(default=0, ge=0),
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """List discovery tasks for a tenant, optionally filtered by status."""
    async with pool.acquire() as conn:
        if status is not None:
            rows = await conn.fetch(
                """SELECT * FROM discovery_tasks
                   WHERE tenant_id = $1 AND status = $2
                   ORDER BY created_at DESC
                   LIMIT $3 OFFSET $4""",
                uuid.UUID(tenant_id),
                status,
                limit,
                offset,
            )
            total = await conn.fetchval(
                "SELECT COUNT(*) FROM discovery_tasks WHERE tenant_id = $1 AND status = $2",
                uuid.UUID(tenant_id),
                status,
            )
        else:
            rows = await conn.fetch(
                """SELECT * FROM discovery_tasks
                   WHERE tenant_id = $1
                   ORDER BY created_at DESC
                   LIMIT $2 OFFSET $3""",
                uuid.UUID(tenant_id),
                limit,
                offset,
            )
            total = await conn.fetchval(
                "SELECT COUNT(*) FROM discovery_tasks WHERE tenant_id = $1",
                uuid.UUID(tenant_id),
            )

    return {"tasks": [dict(r) for r in rows], "total": total}


@router.get("/discovery/tasks/{task_id}")
async def get_task(
    task_id: str,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Get a single discovery task by ID."""
    async with pool.acquire() as conn:
        row = await conn.fetchrow(
            "SELECT * FROM discovery_tasks WHERE id = $1",
            uuid.UUID(task_id),
        )
    if not row:
        raise HTTPException(status_code=404, detail="Task not found")
    return dict(row)


@router.post("/discovery/mac-scan")
async def trigger_mac_scan(
    pool: asyncpg.Pool = Depends(get_db_pool),
    nats_client: NATSClient | None = Depends(get_nats),
):
    """Trigger an immediate MAC/CDP table scan for all network assets."""
    from app.config import settings

    tenant_id = settings.tenant_id
    result = await run_mac_scan(pool, nats_client, tenant_id)
    return {
        "status": "ok",
        "scanned_ips": result.get("scanned_ips", 0),
        "entries_collected": result.get("entries", 0),
    }
