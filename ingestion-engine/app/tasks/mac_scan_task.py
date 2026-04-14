"""Periodic MAC/CDP table scanning and NATS publishing."""

import ipaddress
import logging
from uuid import UUID

import asyncpg
from nats.aio.client import Client as NATSClient

from app.collectors.snmp import SNMPCollector
from app.config import settings
from app.credentials.encryption import decrypt_params, get_key_from_hex
from app.events import publish_event
from app.models.common import CollectTarget

logger = logging.getLogger(__name__)


def _expand_cidr(cidr: str) -> list[str]:
    """Expand a CIDR block to individual IPs."""
    try:
        network = ipaddress.ip_network(cidr, strict=False)
        return [str(ip) for ip in network.hosts()]
    except ValueError:
        # Single IP
        return [cidr.strip()]


async def _scan_single_switch(
    collector: SNMPCollector,
    ip: str,
    cred: dict,
    pool: asyncpg.Pool,
    tenant_id: UUID,
    switch_asset_id: str | None = None,
) -> list[dict]:
    """Scan one switch for CDP neighbors and MAC table."""
    entries: list[dict] = []

    # If no switch_asset_id provided, try to find it from assets
    if not switch_asset_id:
        async with pool.acquire() as conn:
            row = await conn.fetchrow(
                "SELECT id FROM assets WHERE attributes->>'management_ip' = $1 AND tenant_id = $2 AND deleted_at IS NULL",
                ip,
                tenant_id,
            )
        switch_asset_id = str(row["id"]) if row else ""

    target = CollectTarget(endpoint=ip, credentials=cred)

    # CDP neighbors (Cisco)
    try:
        cdp = await collector.collect_cdp_neighbors(target)
        for entry in cdp:
            entries.append({
                "switch_asset_id": switch_asset_id,
                "port_name": entry.get("port_name", ""),
                "mac_address": entry.get("device_mac", ""),
                "device_name": entry.get("device_name", ""),
            })
    except Exception as e:
        logger.debug("CDP failed for %s: %s", ip, e)

    # MAC table (fallback)
    try:
        macs = await collector.collect_mac_table(target)
        existing_macs = {e["mac_address"] for e in entries}
        for entry in macs:
            mac = entry.get("mac_address", "")
            if mac and mac not in existing_macs:
                entries.append({
                    "switch_asset_id": switch_asset_id,
                    "port_name": f"bridge-port-{entry.get('bridge_port', 0)}",
                    "mac_address": mac,
                })
    except Exception as e:
        logger.debug("MAC table failed for %s: %s", ip, e)

    return entries


async def run_mac_scan(
    pool: asyncpg.Pool,
    nats_client: NATSClient | None,
    tenant_id: str,
) -> dict:
    """Scan switches from both scan_targets and assets table.

    Returns a dict with scanned_ips count and entries count.
    """
    tid = UUID(tenant_id)
    enc_key = get_key_from_hex(settings.credential_encryption_key)
    collector = SNMPCollector()
    all_entries: list[dict] = []
    seen_ips: set[str] = set()

    # Source 1: scan_targets (explicit config, has credential_id)
    async with pool.acquire() as conn:
        scan_targets = await conn.fetch(
            """SELECT st.cidrs, c.params as cred_params
               FROM scan_targets st
               JOIN credentials c ON st.credential_id = c.id
               WHERE st.tenant_id = $1 AND st.collector_type = 'snmp'""",
            tid,
        )

    for st in scan_targets:
        cred = decrypt_params(bytes(st["cred_params"]), enc_key)
        for cidr in st["cidrs"]:
            ips = _expand_cidr(cidr)
            for ip in ips:
                if ip in seen_ips:
                    continue
                seen_ips.add(ip)
                entries = await _scan_single_switch(collector, ip, cred, pool, tid)
                all_entries.extend(entries)

    # Source 2: assets table (switches with management_ip)
    # Use first available SNMP credential as fallback
    async with pool.acquire() as conn:
        default_cred_row = await conn.fetchrow(
            "SELECT params FROM credentials WHERE tenant_id = $1 AND type IN ('snmp_v2c', 'snmp_v3') LIMIT 1",
            tid,
        )

    if default_cred_row:
        default_cred = decrypt_params(bytes(default_cred_row["params"]), enc_key)

        async with pool.acquire() as conn:
            switches = await conn.fetch(
                """SELECT id, attributes->>'management_ip' as mgmt_ip
                   FROM assets
                   WHERE tenant_id = $1 AND type = 'network' AND deleted_at IS NULL
                   AND attributes->>'management_ip' IS NOT NULL""",
                tid,
            )

        for sw in switches:
            ip = sw["mgmt_ip"]
            if not ip or ip in seen_ips:
                continue
            seen_ips.add(ip)
            entries = await _scan_single_switch(collector, ip, default_cred, pool, tid, str(sw["id"]))
            all_entries.extend(entries)

    if not all_entries:
        logger.debug("No MAC entries collected from %d IPs", len(seen_ips))
        return {"scanned_ips": len(seen_ips), "entries": 0}

    # Publish to NATS
    if nats_client:
        await publish_event(nats_client, "mac_table.updated", tenant_id, {
            "tenant_id": tenant_id,
            "entries": all_entries,
        })
        logger.info("Published %d MAC entries from %d IPs", len(all_entries), len(seen_ips))

    return {"scanned_ips": len(seen_ips), "entries": len(all_entries)}
