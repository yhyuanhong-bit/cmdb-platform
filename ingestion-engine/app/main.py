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


@asynccontextmanager
async def lifespan(application: FastAPI):
    """Manage application startup and shutdown."""
    # Startup
    logger.info("Starting ingestion engine...")
    application.state.db_pool = await create_pool(settings.database_url)
    logger.info("Database pool created")

    application.state.nats_client = await connect_nats(settings.nats_url)

    yield

    # Shutdown
    logger.info("Shutting down ingestion engine...")
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
