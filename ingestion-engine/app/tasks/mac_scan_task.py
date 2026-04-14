"""Periodic MAC/CDP table scanning and NATS publishing."""

import logging
from uuid import UUID

import asyncpg
from nats.aio.client import Client as NATSClient

from app.collectors.snmp import SNMPCollector
from app.config import settings
from app.events import publish_event
from app.models.common import CollectTarget

logger = logging.getLogger(__name__)


async def run_mac_scan(
    pool: asyncpg.Pool,
    nats_client: NATSClient | None,
    tenant_id: str,
) -> int:
    """Scan all switches for MAC/CDP tables and publish results to NATS.

    Returns the number of entries collected.
    """
    tenant_uuid = UUID(tenant_id)

    # 1. Find all network assets with a management IP
    async with pool.acquire() as conn:
        rows = await conn.fetch(
            """SELECT id, attributes->>'management_ip' AS mgmt_ip
               FROM assets
               WHERE tenant_id = $1
                 AND type = 'network'
                 AND deleted_at IS NULL
                 AND attributes->>'management_ip' IS NOT NULL""",
            tenant_uuid,
        )

    if not rows:
        logger.debug("No switches found for MAC scan in tenant %s", tenant_id)
        return 0

    # 2. Get first available SNMP credential for this tenant
    async with pool.acquire() as conn:
        cred_row = await conn.fetchrow(
            "SELECT params FROM credentials WHERE tenant_id = $1 AND type = 'snmp_v2c' LIMIT 1",
            tenant_uuid,
        )

    if not cred_row:
        logger.warning("No SNMP credentials configured for tenant %s", tenant_id)
        return 0

    # Decrypt credential params
    from app.credentials.encryption import decrypt_params

    cred_params = decrypt_params(cred_row["params"])

    collector = SNMPCollector()
    all_entries: list[dict] = []

    for row in rows:
        switch_id = str(row["id"])
        mgmt_ip = row["mgmt_ip"]
        if not mgmt_ip:
            continue

        target = CollectTarget(
            endpoint=mgmt_ip,
            credentials=cred_params,
        )

        # Collect CDP neighbors
        try:
            cdp_entries = await collector.collect_cdp_neighbors(target)
            for entry in cdp_entries:
                all_entries.append({
                    "switch_asset_id": switch_id,
                    "port_name": entry.get("port_name", ""),
                    "mac_address": "",
                    "device_name": entry.get("device_name", ""),
                })
        except Exception:
            logger.exception("CDP collection error for switch %s (%s)", switch_id, mgmt_ip)

        # Collect MAC table
        try:
            mac_entries = await collector.collect_mac_table(target)
            for entry in mac_entries:
                mac = entry.get("mac_address", "")
                all_entries.append({
                    "switch_asset_id": switch_id,
                    "port_name": f"bridge-port-{entry.get('bridge_port', 0)}",
                    "mac_address": mac,
                    "device_name": "",
                })
        except Exception:
            logger.exception("MAC collection error for switch %s (%s)", switch_id, mgmt_ip)

    if not all_entries:
        logger.debug("No MAC entries collected for tenant %s", tenant_id)
        return 0

    # 3. Publish to NATS via the standard event wrapper
    await publish_event(
        nats_client,
        "mac_table.updated",
        tenant_id,
        {"entries": all_entries},
    )
    logger.info("Published %d MAC entries to NATS for tenant %s", len(all_entries), tenant_id)

    return len(all_entries)
