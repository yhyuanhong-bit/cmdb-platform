"""Celery task for async import processing with progress tracking."""

import asyncio
import json
import logging
from datetime import datetime, timezone
from uuid import UUID

import asyncpg

from app.config import settings
from app.importers.excel_parser import parse_csv, parse_excel, rows_to_raw_assets
from app.pipeline.processor import process_batch
from app.tasks.celery_app import celery_app

logger = logging.getLogger(__name__)


def _run_async(coro):
    """Create a new event loop, run the coroutine, and close the loop."""
    loop = asyncio.new_event_loop()
    try:
        return loop.run_until_complete(coro)
    finally:
        loop.close()


# Retryable infrastructure errors for process_import_task.
#
# The task only hits asyncpg (Postgres) and reads a local file. asyncpg.PostgresError
# covers transient DB failures (connection resets, lock timeouts, temporary
# unavailability). OSError covers network-level socket issues that surface below
# asyncpg (DNS, ECONNREFUSED, ETIMEDOUT) and transient filesystem glitches on the
# uploaded file. We deliberately do NOT retry on ValueError / validation errors —
# those are deterministic and would just burn retry budget.
_IMPORT_RETRYABLE_EXCEPTIONS = (asyncpg.PostgresError, OSError)


@celery_app.task(
    bind=True,
    name="ingestion.process_import",
    autoretry_for=_IMPORT_RETRYABLE_EXCEPTIONS,
    retry_backoff=True,
    retry_backoff_max=600,
    max_retries=5,
)
def process_import_task(self, import_job_id: str, tenant_id: str, file_path: str, file_type: str):
    """Celery task that processes an import file through the pipeline."""
    return _run_async(_process_import(self, import_job_id, tenant_id, file_path, file_type))


async def _process_import(task, import_job_id: str, tenant_id: str, file_path: str, file_type: str):
    """Async implementation of the import processing."""
    pool = await asyncpg.create_pool(settings.database_url, min_size=2, max_size=10)
    try:
        await _update_job_status(pool, import_job_id, "processing")

        # Parse file
        if file_type in ("xlsx", "excel"):
            result = parse_excel(file_path)
        else:
            result = parse_csv(file_path)

        await _update_job_total(pool, import_job_id, result.total_rows)

        # Convert to RawAssetData
        source = "excel" if file_type in ("xlsx", "excel") else "csv"
        raw_assets = rows_to_raw_assets(result.valid_rows, source=source)

        # Progress callback
        def on_progress(processed: int, total: int):
            _run_async(_update_job_progress(pool, import_job_id, processed))
            task.update_state(
                state="PROGRESS",
                meta={"processed": processed, "total": total},
            )

        # Process through pipeline
        stats = await process_batch(
            pool,
            UUID(tenant_id),
            raw_assets,
            progress_callback=on_progress,
        )

        # Build error details from error rows
        error_details = []
        for row in result.error_rows:
            error_details.append({
                "row": row.row_num,
                "errors": row.errors or [],
                "data": row.data,
            })

        await _update_job_completed(pool, import_job_id, stats, error_details)

        return {"status": "completed", "stats": stats}

    except Exception as e:
        logger.exception("Import task failed for job %s", import_job_id)
        await _update_job_status(pool, import_job_id, "failed")
        raise
    finally:
        await pool.close()


async def _update_job_status(pool: asyncpg.Pool, job_id: str, status: str):
    """Update the status of an import job."""
    async with pool.acquire() as conn:
        await conn.execute(
            "UPDATE import_jobs SET status = $1 WHERE id = $2",
            status, UUID(job_id),
        )


async def _update_job_total(pool: asyncpg.Pool, job_id: str, total: int):
    """Update the total_rows of an import job."""
    async with pool.acquire() as conn:
        await conn.execute(
            "UPDATE import_jobs SET total_rows = $1 WHERE id = $2",
            total, UUID(job_id),
        )


async def _update_job_progress(pool: asyncpg.Pool, job_id: str, processed: int):
    """Update the processed_rows count of an import job."""
    async with pool.acquire() as conn:
        await conn.execute(
            "UPDATE import_jobs SET processed_rows = $1 WHERE id = $2",
            processed, UUID(job_id),
        )


async def _update_job_completed(pool: asyncpg.Pool, job_id: str, stats: dict, error_details: list):
    """Mark an import job as completed with final stats."""
    async with pool.acquire() as conn:
        await conn.execute(
            """UPDATE import_jobs
               SET status = 'completed',
                   processed_rows = total_rows,
                   stats = $1,
                   error_details = $2,
                   completed_at = $3
               WHERE id = $4""",
            json.dumps(stats),
            json.dumps(error_details),
            datetime.now(timezone.utc),
            UUID(job_id),
        )
