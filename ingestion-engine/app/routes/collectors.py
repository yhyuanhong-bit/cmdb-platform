"""Collector management routes."""

from dataclasses import asdict
from uuid import UUID

import asyncpg
from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel

from app.collectors.manager import manager
from app.credentials.provider import DBCredentialProvider
from app.dependencies import get_db_pool
from app.models.common import CollectTarget

router = APIRouter(tags=["collectors"])


class TestRequest(BaseModel):
    credential_id: str
    endpoint: str


@router.get("/collectors")
async def list_collectors():
    """List all registered collectors with their status."""
    return {"collectors": manager.list_all()}


@router.post("/collectors/{name}/start")
async def start_collector(name: str):
    """Start a collector by name."""
    try:
        status = manager.start(name)
        return asdict(status)
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))


@router.post("/collectors/{name}/stop")
async def stop_collector(name: str):
    """Stop a collector by name."""
    try:
        status = manager.stop(name)
        return asdict(status)
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))


@router.post("/collectors/{name}/test")
async def test_collector(name: str, body: TestRequest, pool: asyncpg.Pool = Depends(get_db_pool)):
    """Test connectivity for a collector using a stored credential."""
    provider = DBCredentialProvider(pool)
    try:
        cred = await provider.get(UUID(body.credential_id))
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))
    target = CollectTarget(endpoint=body.endpoint, credentials=cred["params"])
    try:
        result = await manager.test_connection(name, target)
        return result.model_dump()
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))
