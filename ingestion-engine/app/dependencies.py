import asyncpg
from fastapi import Request
from nats.aio.client import Client as NATSClient


async def get_db_pool(request: Request) -> asyncpg.Pool:
    return request.app.state.db_pool


async def get_nats(request: Request) -> NATSClient | None:
    return request.app.state.nats_client
