import json
import logging
from datetime import datetime, timezone

import nats
from nats.aio.client import Client as NATSClient

logger = logging.getLogger(__name__)


async def connect_nats(url: str) -> NATSClient | None:
    """Connect to NATS server. Returns None on failure."""
    try:
        nc = await nats.connect(url)
        logger.info("Connected to NATS at %s", url)
        return nc
    except Exception as e:
        logger.warning("Failed to connect to NATS at %s: %s", url, e)
        return None


async def close_nats(nc: NATSClient | None) -> None:
    """Close the NATS connection if active."""
    if nc and not nc.is_closed:
        await nc.close()
        logger.info("NATS connection closed")


async def publish_event(
    nc: NATSClient | None,
    subject: str,
    tenant_id: str,
    payload_dict: dict,
) -> None:
    """Publish an event to a NATS subject."""
    if nc is None or nc.is_closed:
        logger.warning("NATS not connected, skipping publish to %s", subject)
        return

    message = {
        "tenant_id": tenant_id,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "payload": payload_dict,
    }
    await nc.publish(subject, json.dumps(message).encode())
