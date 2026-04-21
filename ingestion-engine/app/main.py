import asyncio
import logging
from contextlib import asynccontextmanager

import redis.asyncio as redis
from fastapi import FastAPI

from app.config import settings
from app.database import close_pool, create_pool
from app.events import close_nats, connect_nats
from app.leader import default_node_id, try_acquire
from app.routes.collectors import router as collectors_router
from app.routes.conflicts import router as conflicts_router
from app.routes.credentials import router as credentials_router
from app.routes.discovery import router as discovery_router
from app.routes.imports import router as imports_router
from app.routes.scan_targets import router as scan_targets_router

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

MAC_SCAN_INTERVAL_SECONDS = 300  # 5 minutes
# Leader lease must outlive one scan cycle so we don't hand off mid-run.
# Also kept short enough that a crashed leader is replaced within ~2min
# of its death.
MAC_SCAN_LEASE_SECONDS = MAC_SCAN_INTERVAL_SECONDS + 120  # 7 minutes
MAC_SCAN_LOCK_KEY = "ingestion:lock:mac-scan"


async def _periodic_mac_scan(application: FastAPI) -> None:
    """Background loop that runs MAC table scanning every 5 minutes.

    Multi-replica safe: every replica enters this loop, but only the one
    that currently holds the Redis lease actually performs the scan. A
    dead leader's lease expires within MAC_SCAN_LEASE_SECONDS so another
    replica picks up without operator intervention.
    """
    await asyncio.sleep(60)  # Wait 1 min after startup for services to stabilise
    node_id = default_node_id()
    redis_client = application.state.redis_client
    while True:
        try:
            is_leader = await try_acquire(
                redis_client, MAC_SCAN_LOCK_KEY, node_id, MAC_SCAN_LEASE_SECONDS
            )
            if not is_leader:
                # Another replica owns this cycle. Silent on purpose —
                # every-5-minutes INFO log from every follower replica
                # would spam operator dashboards.
                logger.debug("MAC scan skipped: lease held by another replica")
            else:
                from app.tasks.mac_scan_task import run_mac_scan

                pool = application.state.db_pool
                nats_client = application.state.nats_client
                result = await run_mac_scan(pool, nats_client, settings.tenant_id)
                logger.info(
                    "Periodic MAC scan completed (leader=%s): scanned_ips=%s entries=%s",
                    node_id,
                    result.get("scanned_ips", 0),
                    result.get("entries", 0),
                )
        except Exception:
            logger.warning("Periodic MAC scan failed", exc_info=True)
        await asyncio.sleep(MAC_SCAN_INTERVAL_SECONDS)


@asynccontextmanager
async def lifespan(application: FastAPI):
    """Manage application startup and shutdown."""
    # Startup
    logger.info("Starting ingestion engine...")
    application.state.db_pool = await create_pool(settings.database_url)
    logger.info("Database pool created")

    application.state.nats_client = await connect_nats(settings.nats_url)

    # Shared Redis client is used by the MAC-scan leader lease. Celery
    # already uses Redis as broker/backend, so we're not adding a new
    # infrastructure dependency here.
    application.state.redis_client = redis.from_url(settings.redis_url, decode_responses=True)

    # Start periodic MAC scan background task
    scan_task = asyncio.create_task(_periodic_mac_scan(application))
    logger.info("Periodic MAC scan scheduled (every %ds, leader-elected via Redis)",
                MAC_SCAN_INTERVAL_SECONDS)

    yield

    # Shutdown
    logger.info("Shutting down ingestion engine...")
    scan_task.cancel()
    try:
        await scan_task
    except asyncio.CancelledError:
        pass
    await application.state.redis_client.aclose()
    await close_nats(application.state.nats_client)
    await close_pool(application.state.db_pool)
    logger.info("Shutdown complete")


def create_app() -> FastAPI:
    """Create and configure the FastAPI application."""
    application = FastAPI(
        title="CMDB Ingestion Engine",
        version="0.1.0",
        lifespan=lifespan,
    )

    application.include_router(imports_router)
    application.include_router(conflicts_router)
    application.include_router(collectors_router)
    application.include_router(credentials_router)
    application.include_router(scan_targets_router)
    application.include_router(discovery_router)

    @application.get("/healthz")
    async def healthz():
        return {"status": "ok"}

    return application


app = create_app()
