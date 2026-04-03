"""Import routes: upload, preview, confirm, progress, templates."""

import json
import logging
import os
import uuid
from pathlib import Path

import asyncpg
from fastapi import APIRouter, Depends, HTTPException, UploadFile
from fastapi.responses import StreamingResponse

from app.config import settings
from app.dependencies import get_db_pool
from app.importers.excel_parser import parse_csv, parse_excel
from app.importers.templates import generate_asset_template
from app.tasks.import_task import process_import_task

logger = logging.getLogger(__name__)

router = APIRouter(tags=["imports"])


def _find_file(job_id: str, filename: str) -> str | None:
    """Scan the upload directory for a file matching the job_id prefix."""
    upload_dir = Path(settings.upload_dir)
    if not upload_dir.exists():
        return None
    for entry in upload_dir.iterdir():
        if entry.is_file() and entry.name.startswith(job_id):
            return str(entry)
    # Also try matching by filename directly
    candidate = upload_dir / f"{job_id}_{filename}"
    if candidate.exists():
        return str(candidate)
    return None


@router.post("/import/upload")
async def upload_import(
    file: UploadFile,
    tenant_id: str,
    uploaded_by: str | None = None,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Accept an uploaded file, parse for preview, create import job."""
    # Ensure upload directory exists
    os.makedirs(settings.upload_dir, exist_ok=True)

    job_id = str(uuid.uuid4())
    original_filename = file.filename or "upload"
    ext = original_filename.rsplit(".", 1)[-1].lower() if "." in original_filename else "xlsx"
    saved_filename = f"{job_id}_{original_filename}"
    file_path = os.path.join(settings.upload_dir, saved_filename)

    # Save file
    content = await file.read()
    with open(file_path, "wb") as f:
        f.write(content)

    # Parse for preview
    try:
        if ext in ("xlsx", "xls"):
            result = parse_excel(file_path)
        else:
            result = parse_csv(file_path)
    except Exception as e:
        logger.exception("Failed to parse uploaded file")
        raise HTTPException(status_code=400, detail=f"Failed to parse file: {e}")

    stats = {
        "total_rows": result.total_rows,
        "valid_rows": len(result.valid_rows),
        "error_rows": len(result.error_rows),
    }

    preview = [row.model_dump() for row in result.preview]
    errors = [row.model_dump() for row in result.error_rows]

    # Create import_jobs record
    async with pool.acquire() as conn:
        await conn.execute(
            """INSERT INTO import_jobs (id, tenant_id, type, filename, status, total_rows,
                   processed_rows, stats, error_details, uploaded_by)
               VALUES ($1, $2, $3, $4, 'previewing', $5, 0, $6, $7, $8)""",
            uuid.UUID(job_id),
            uuid.UUID(tenant_id),
            ext,
            original_filename,
            result.total_rows,
            json.dumps(stats),
            json.dumps(errors),
            uuid.UUID(uploaded_by) if uploaded_by else None,
        )

    return {
        "job_id": job_id,
        "stats": stats,
        "preview": preview,
        "errors": errors,
    }


@router.get("/import/{job_id}/preview")
async def get_preview(
    job_id: str,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Fetch the preview data for an import job."""
    async with pool.acquire() as conn:
        row = await conn.fetchrow(
            "SELECT * FROM import_jobs WHERE id = $1",
            uuid.UUID(job_id),
        )
    if not row:
        raise HTTPException(status_code=404, detail="Import job not found")
    return dict(row)


@router.post("/import/{job_id}/confirm")
async def confirm_import(
    job_id: str,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Confirm an import job and dispatch the Celery task."""
    async with pool.acquire() as conn:
        row = await conn.fetchrow(
            "SELECT * FROM import_jobs WHERE id = $1",
            uuid.UUID(job_id),
        )
    if not row:
        raise HTTPException(status_code=404, detail="Import job not found")
    if row["status"] != "previewing":
        raise HTTPException(status_code=400, detail=f"Job status is '{row['status']}', expected 'previewing'")

    # Update status to confirmed
    async with pool.acquire() as conn:
        await conn.execute(
            "UPDATE import_jobs SET status = 'confirmed' WHERE id = $1",
            uuid.UUID(job_id),
        )

    # Find the uploaded file
    file_path = _find_file(job_id, row["filename"])
    if not file_path:
        raise HTTPException(status_code=404, detail="Uploaded file not found")

    # Dispatch Celery task
    process_import_task.delay(
        job_id,
        str(row["tenant_id"]),
        file_path,
        row["type"],
    )

    return {"job_id": job_id, "status": "confirmed", "message": "Import task dispatched"}


@router.get("/import/{job_id}/progress")
async def get_progress(
    job_id: str,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Fetch the current progress of an import job."""
    async with pool.acquire() as conn:
        row = await conn.fetchrow(
            "SELECT id, status, total_rows, processed_rows, stats, error_details, completed_at FROM import_jobs WHERE id = $1",
            uuid.UUID(job_id),
        )
    if not row:
        raise HTTPException(status_code=404, detail="Import job not found")
    return dict(row)


@router.get("/import/templates/{template_type}")
async def download_template(template_type: str):
    """Return a downloadable Excel template."""
    if template_type != "asset":
        raise HTTPException(status_code=404, detail=f"Unknown template type: {template_type}")

    buf = generate_asset_template()
    return StreamingResponse(
        buf,
        media_type="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
        headers={"Content-Disposition": "attachment; filename=asset_import_template.xlsx"},
    )
