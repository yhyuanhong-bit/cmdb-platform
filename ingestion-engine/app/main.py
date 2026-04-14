import asyncio
import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.config import settings
from app.database import close_pool, create_pool
from app.events import close_nats, connect_nats
from app.routes.collectors import router as collectors_router
from app.routes.conflicts import router as conflicts_router
from app.routes.credentials import router as credentials_router
from app.routes.discovery import router as discovery_router
from app.routes.imports import router as imports_router
from app.routes.scan_targets import router as scan_targets_router

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

MAC_SCAN_INTERVAL_SECONDS = 300  # 5 minutes


async def _periodic_mac_scan(application: FastAPI) -> None:
    """Background loop that runs MAC table scanning every 5 minutes."""
    await asyncio.sleep(60)  # Wait 1 min after startup for services to stabilise
    while True:
        try:
            from app.tasks.mac_scan_task import run_mac_scan

            pool = application.state.db_pool
            nats_client = application.state.nats_client
            result = await run_mac_scan(pool, nats_client, settings.tenant_id)
            logger.info(
                "Periodic MAC scan completed: scanned_ips=%s entries=%s",
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

    # Start periodic MAC scan background task
    scan_task = asyncio.create_task(_periodic_mac_scan(application))
    logger.info("Periodic MAC scan scheduled (every %ds)", MAC_SCAN_INTERVAL_SECONDS)

    yield

    # Shutdown
    logger.info("Shutting down ingestion engine...")
    scan_task.cancel()
    try:
        await scan_task
    except asyncio.CancelledError:
        pass
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
