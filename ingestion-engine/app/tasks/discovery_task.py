"""Celery task for discovery scanning with auto/review/smart mode routing."""

import asyncio
import logging
from datetime import datetime, timezone
from uuid import UUID

import asyncpg
import httpx
import nats.errors

from app.collectors.base import registry
from app.config import settings
from app.credentials.provider import DBCredentialProvider
from app.events import close_nats, connect_nats, publish_event
from app.models.common import CollectTarget, RawAssetData
from app.pipeline.deduplicate import deduplicate
from app.pipeline.processor import process_single
from app.tasks.celery_app import celery_app

logger = logging.getLogger(__name__)


def _run_async(coro):
    """Create a new event loop, run the coroutine, and close the loop."""
    loop = asyncio.new_event_loop()
    try:
        return loop.run_until_complete(coro)
    finally:
        loop.close()


def determine_routing(mode: str, raw: RawAssetData, existing_asset_id) -> str:
    """Determine routing destination for a discovered asset.

    Wave 3 changed the safe default. Modes:
      - "auto":   always pipeline (auto-merge / auto-create).  Requires
                  explicit opt-in; a scanner misconfigured here pollutes
                  the CMDB silently.
      - "review": always staging (human must approve).  Default.
      - "smart":  pipeline when a matching CI already exists, staging
                  when the discovery would create a new CI.  Pre-3
                  default; kept for back-compat.

    Args:
        mode: One of "auto", "review", or "smart"
        raw: The raw asset data (unused in routing logic, reserved for future use)
        existing_asset_id: UUID of existing asset if found, else None

    Returns:
        "pipeline" or "staging"
    """
    if mode == "auto":
        return "pipeline"
    if mode == "review":
        return "staging"
    # smart
    if existing_asset_id:
        return "pipeline"
    return "staging"


def compute_match_metadata(raw: RawAssetData, existing_asset_id) -> dict:
    """Derive match_confidence + match_strategy for a staged discovery.

    Wave 3 surfaces these to the review UI so operators can tell whether
    the dedup step matched on a strong identifier (serial_number, exact
    asset_tag) versus a weaker signal (hostname / IP only).

    Returns a dict suitable for JSON-posting to /discovery/ingest. Keys
    are omitted when no match was made — NULL passthrough on the backend.
    """
    if existing_asset_id is None:
        return {}

    # Strategy order mirrors deduplicate.py's lookup preference. We
    # re-derive the strategy here from the raw fields because the
    # deduplicate result doesn't currently return which field hit.
    if raw.fields.get("serial_number"):
        return {"match_confidence": 0.95, "match_strategy": "serial_number"}
    if raw.fields.get("asset_tag"):
        return {"match_confidence": 0.90, "match_strategy": "asset_tag"}
    if raw.fields.get("name") or raw.fields.get("hostname"):
        return {"match_confidence": 0.70, "match_strategy": "hostname"}
    if (raw.attributes or {}).get("ip_address"):
        return {"match_confidence": 0.60, "match_strategy": "ip"}
    # Matched by some other path — surface a low score so the reviewer
    # treats it as "inspect before approving".
    return {"match_confidence": 0.50, "match_strategy": "hostname"}


# Retryable infrastructure errors for process_discovery_task.
#
# Discovery hits three external systems: Postgres (asyncpg), cmdb-core HTTP
# endpoint for staging (httpx), and NATS for event publication. Each has a
# specific transient-failure class that should retry with backoff. OSError
# catches low-level socket issues under any of them (DNS, connection refused,
# network unreachable). Deterministic errors like ValueError (unknown collector)
# and asyncio.CancelledError are left to fail fast.
_DISCOVERY_RETRYABLE_EXCEPTIONS = (
    httpx.HTTPError,
    asyncpg.PostgresError,
    nats.errors.Error,
    OSError,
)


@celery_app.task(
    bind=True,
    name="ingestion.process_discovery",
    autoretry_for=_DISCOVERY_RETRYABLE_EXCEPTIONS,
    retry_backoff=True,
    retry_backoff_max=600,
    max_retries=5,
)
def process_discovery_task(
    self,
    task_id: str,
    tenant_id: str,
    collector_type: str,
    cidrs: list[str],
    credential_id: str,
    mode: str,
):
    """Celery task that runs a discovery scan through the pipeline or staging."""
    return _run_async(
        _process_discovery(self, task_id, tenant_id, collector_type, cidrs, credential_id, mode)
    )


async def _process_discovery(
    task,
    task_id: str,
    tenant_id: str,
    collector_type: str,
    cidrs: list[str],
    credential_id: str,
    mode: str,
):
    """Async implementation of discovery processing."""
    pool = await asyncpg.create_pool(settings.database_url, min_size=2, max_size=10)
    nc = await connect_nats(settings.nats_url)

    stats = {
        "total_ips": 0,
        "responded": 0,
        "created": 0,
        "updated": 0,
        "conflicts": 0,
        "skipped": 0,
        "staging": 0,
        "errors": 0,
    }

    try:
        # Mark task as running
        await _update_task_status(pool, task_id, "running")

        # Load credential
        provider = DBCredentialProvider(pool)
        credential = await provider.get(UUID(credential_id))

        # Get collector
        collector = registry.get(collector_type)
        if collector is None:
            raise ValueError(f"Collector '{collector_type}' not found in registry")

        tenant_uuid = UUID(tenant_id)

        # Collect from each CIDR
        for cidr in cidrs:
            stats["total_ips"] += 1
            try:
                target = CollectTarget(
                    endpoint=cidr,
                    credentials=credential.get("params"),
                )
                results: list[RawAssetData] = await collector.collect(target)

                for raw in results:
                    stats["responded"] += 1
                    try:
                        # Deduplicate check
                        dedup_result = await deduplicate(pool, tenant_uuid, raw)
                        route = determine_routing(mode, raw, dedup_result.existing_asset_id)

                        if route == "pipeline":
                            result = await process_single(pool, tenant_uuid, raw)
                            if result.action == "created":
                                stats["created"] += 1
                            elif result.action == "updated":
                                stats["updated"] += 1
                            elif result.action == "conflict":
                                stats["conflicts"] += 1
                            else:
                                stats["skipped"] += 1
                        else:
                            match_meta = compute_match_metadata(
                                raw, dedup_result.existing_asset_id
                            )
                            await _send_to_staging(
                                tenant_id,
                                raw,
                                dedup_result.existing_asset_id,
                                match_meta,
                            )
                            stats["staging"] += 1

                    except Exception:
                        logger.exception(
                            "Error processing result from CIDR %s for task %s", cidr, task_id
                        )
                        stats["errors"] += 1

            except Exception:
                logger.exception("Error collecting from CIDR %s for task %s", cidr, task_id)
                stats["errors"] += 1

        # Mark task completed with stats
        await _update_task_completed(pool, task_id, stats)

        # Publish NATS event
        await publish_event(
            nc,
            "import.completed",
            tenant_id,
            {"task_id": task_id, "stats": stats},
        )

        return {"status": "completed", "stats": stats}

    except Exception:
        logger.exception("Discovery task failed for task %s", task_id)
        await _update_task_status(pool, task_id, "failed")
        raise
    finally:
        await pool.close()
        await close_nats(nc)


async def _send_to_staging(
    tenant_id: str,
    raw: RawAssetData,
    existing_asset_id=None,
    match_meta: dict | None = None,
) -> None:
    """POST a discovered asset to cmdb-core /discovery/ingest for human review.

    Wave 3: when the dedup step found a matching CI, we pass through
    matched_asset_id plus the confidence/strategy pair so the review UI
    can show "we think this is asset X with 95% confidence (matched by
    serial_number)" instead of forcing the reviewer to eyeball the diff.
    """
    payload = {
        "source": raw.source,
        "hostname": raw.fields.get("name", ""),
        "ip_address": (raw.attributes or {}).get("ip_address", ""),
        "raw_data": {"fields": raw.fields, "attributes": raw.attributes or {}},
    }
    if existing_asset_id is not None:
        payload["matched_asset_id"] = str(existing_asset_id)
    if match_meta:
        payload.update(match_meta)

    async with httpx.AsyncClient() as client:
        resp = await client.post(
            f"{settings.cmdb_core_url}/discovery/ingest",
            json=payload,
            headers={"X-Tenant-ID": tenant_id},
            timeout=10,
        )
        resp.raise_for_status()


async def _update_task_status(pool: asyncpg.Pool, task_id: str, status: str) -> None:
    """Update discovery_tasks status."""
    async with pool.acquire() as conn:
        await conn.execute(
            "UPDATE discovery_tasks SET status = $1 WHERE id = $2",
            status,
            UUID(task_id),
        )


async def _update_task_completed(pool: asyncpg.Pool, task_id: str, stats: dict) -> None:
    """Mark discovery_tasks as completed with stats and completed_at timestamp."""
    import json

    async with pool.acquire() as conn:
        await conn.execute(
            """UPDATE discovery_tasks
               SET status = 'completed',
                   stats = $1,
                   completed_at = $2
               WHERE id = $3""",
            json.dumps(stats),
            datetime.now(timezone.utc),
            UUID(task_id),
        )
