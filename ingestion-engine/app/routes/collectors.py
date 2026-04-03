"""Collector management routes."""

from dataclasses import asdict

from fastapi import APIRouter, HTTPException

from app.collectors.manager import manager
from app.models.common import CollectTarget

router = APIRouter(tags=["collectors"])


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
async def test_collector(name: str, target: CollectTarget):
    """Test connectivity for a collector."""
    try:
        result = await manager.test_connection(name, target)
        return result.model_dump()
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))
