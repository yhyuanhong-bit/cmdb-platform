"""IPMI collector using pyghmi for out-of-band hardware discovery."""

import asyncio
import ipaddress
import logging
from datetime import datetime, timezone

from pyghmi.ipmi.command import Command

from app.collectors.base import Collector, registry
from app.models.common import CollectTarget, ConnectionResult, FieldMapping, RawAssetData

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Helper: CIDR expansion
# ---------------------------------------------------------------------------

def expand_cidrs(endpoint: str) -> list[str]:
    """Expand a comma-separated list of IPs and/or CIDRs into individual IPs."""
    ips: list[str] = []
    for part in endpoint.split(","):
        part = part.strip()
        if not part:
            continue
        try:
            network = ipaddress.ip_network(part, strict=False)
            ips.extend(str(host) for host in network.hosts())
        except ValueError:
            # Treat as a plain IP / hostname
            ips.append(part)
    return ips


# ---------------------------------------------------------------------------
# FRU parsing
# ---------------------------------------------------------------------------

def parse_fru_inventory(fru: dict) -> tuple[dict, dict]:
    """Parse a pyghmi FRU dict into (fields, attributes).

    Returns:
        fields: dict with standard field names (serial_number, vendor, model)
        attributes: dict with extended fields (part_number, raw FRU data)
    """
    fields: dict[str, str | None] = {}
    attributes: dict = {}

    # serial_number
    serial = (
        fru.get("product_serial")
        or fru.get("serial_number")
        or fru.get("Serial Number")
    )
    if serial:
        fields["serial_number"] = str(serial).strip()

    # vendor
    vendor = (
        fru.get("product_manufacturer")
        or fru.get("manufacturer")
        or fru.get("Manufacturer")
    )
    if vendor:
        fields["vendor"] = str(vendor).strip()

    # model
    model = fru.get("product_name") or fru.get("Product Name")
    if model:
        fields["model"] = str(model).strip()

    # part_number → attributes
    part_number = fru.get("product_part_number") or fru.get("Part Number")
    if part_number:
        attributes["part_number"] = str(part_number).strip()

    return fields, attributes


# ---------------------------------------------------------------------------
# Synchronous single-host collection (runs in a thread)
# ---------------------------------------------------------------------------

def _collect_single_sync(
    ip: str,
    credentials: dict,
    options: dict,
) -> RawAssetData | None:
    """Collect IPMI data from a single BMC.  Runs synchronously in a thread."""

    username = credentials.get("username", "admin")
    password = credentials.get("password", "admin")
    port = int(options.get("port", 623))

    try:
        conn = Command(bmc=ip, userid=username, password=password, port=port)
    except Exception as exc:
        logger.debug("IPMI connection failed for %s: %s", ip, exc)
        return None

    fields: dict[str, str | None] = {}
    attributes: dict = {}

    # --- FRU inventory ---
    try:
        fru_data = conn.get_inventory()
        if isinstance(fru_data, dict):
            fru_fields, fru_attrs = parse_fru_inventory(fru_data)
            fields.update(fru_fields)
            attributes.update(fru_attrs)
    except Exception as exc:
        logger.debug("IPMI get_inventory failed for %s: %s", ip, exc)

    # --- Chassis / power state ---
    try:
        chassis = conn.get_chassis_status()
        if isinstance(chassis, dict):
            power = chassis.get("power_state") or chassis.get("powerstate")
            if power is not None:
                fields["power_state"] = str(power)
    except Exception as exc:
        logger.debug("IPMI get_chassis_status failed for %s: %s", ip, exc)

    # --- Network configuration ---
    try:
        net_cfg = conn.get_net_configuration()
        if isinstance(net_cfg, dict):
            bmc_ip = net_cfg.get("ipv4_address") or net_cfg.get("ip")
            bmc_mac = net_cfg.get("mac_address") or net_cfg.get("mac")
            if bmc_ip:
                fields["bmc_ip"] = str(bmc_ip)
            if bmc_mac:
                fields["bmc_mac"] = str(bmc_mac)
    except Exception as exc:
        logger.debug("IPMI get_net_configuration failed for %s: %s", ip, exc)

    # --- Sensor data ---
    try:
        sensors = conn.get_sensor_data()
        if sensors:
            sensor_summary: dict = {}
            if isinstance(sensors, dict):
                sensor_summary = {k: str(v) for k, v in sensors.items()}
            elif isinstance(sensors, list):
                sensor_summary = {
                    s.get("name", f"sensor_{i}"): str(s.get("value", ""))
                    for i, s in enumerate(sensors)
                    if isinstance(s, dict)
                }
            if sensor_summary:
                attributes["sensors"] = sensor_summary
    except Exception as exc:
        logger.debug("IPMI get_sensor_data failed for %s: %s", ip, exc)

    # --- Firmware version ---
    try:
        fw = conn.get_firmware()
        if fw:
            if isinstance(fw, dict):
                fw_ver = fw.get("version") or fw.get("firmware_version")
            else:
                fw_ver = str(fw)
            if fw_ver:
                fields["firmware_version"] = str(fw_ver)
    except Exception as exc:
        logger.debug("IPMI get_firmware failed for %s: %s", ip, exc)

    # Use serial_number as unique key if available, else fall back to BMC IP
    unique_key = fields.get("serial_number") or ip

    return RawAssetData(
        source="ipmi",
        unique_key=unique_key,
        fields=fields,
        attributes=attributes if attributes else None,
        collected_at=datetime.now(timezone.utc),
    )


# ---------------------------------------------------------------------------
# Async IPMI Collector
# ---------------------------------------------------------------------------

async def get_bmc_targets_from_assets(pool, tenant_id: str) -> list[dict]:
    """Get BMC scan targets from assets table where bmc_ip is set."""
    async with pool.acquire() as conn:
        rows = await conn.fetch(
            """SELECT id, asset_tag, name, bmc_ip, bmc_type
               FROM assets
               WHERE tenant_id = $1 AND bmc_ip IS NOT NULL AND deleted_at IS NULL""",
            tenant_id,
        )
        return [dict(row) for row in rows]


async def update_bmc_firmware(pool, asset_id: str, firmware_version: str) -> None:
    """Update asset BMC firmware version after a successful IPMI scan."""
    async with pool.acquire() as conn:
        await conn.execute(
            "UPDATE assets SET bmc_firmware = $1, updated_at = now() WHERE id = $2",
            firmware_version,
            asset_id,
        )


class IPMICollector:
    """Out-of-band hardware collector via IPMI/BMC (pyghmi)."""

    name: str = "ipmi"
    collect_type: str = "ipmi"

    def supported_fields(self) -> list[FieldMapping]:
        return [
            FieldMapping(field_name="serial_number", authority=True),
            FieldMapping(field_name="vendor", authority=True),
            FieldMapping(field_name="model", authority=True),
            FieldMapping(field_name="power_state", authority=True),
            FieldMapping(field_name="bmc_ip", authority=False),
            FieldMapping(field_name="bmc_mac", authority=False),
            FieldMapping(field_name="firmware_version", authority=False),
        ]

    async def collect(self, target: CollectTarget, pool=None, tenant_id: str | None = None) -> list[RawAssetData]:
        credentials = target.credentials or {}
        options = target.options or {}
        concurrency = int(options.get("concurrency", 20))
        semaphore = asyncio.Semaphore(concurrency)

        ips = expand_cidrs(target.endpoint)

        # Merge asset-derived BMC targets so known assets are always scanned
        asset_ip_map: dict[str, dict] = {}
        if pool is not None and tenant_id is not None:
            try:
                asset_rows = await get_bmc_targets_from_assets(pool, tenant_id)
                for row in asset_rows:
                    bmc_ip = row.get("bmc_ip")
                    if bmc_ip and bmc_ip not in ips:
                        ips.append(bmc_ip)
                    if bmc_ip:
                        asset_ip_map[bmc_ip] = row
            except Exception as exc:
                logger.warning("Failed to load BMC targets from assets: %s", exc)

        results: list[RawAssetData] = []

        async def _scan(ip: str) -> None:
            async with semaphore:
                raw = await asyncio.to_thread(
                    _collect_single_sync, ip, credentials, options
                )
                if raw is None:
                    return
                results.append(raw)
                # If this IP belongs to a known asset, write firmware back
                if pool is not None and ip in asset_ip_map:
                    fw = raw.fields.get("firmware_version")
                    if fw:
                        asset_row = asset_ip_map[ip]
                        try:
                            await update_bmc_firmware(pool, str(asset_row["id"]), fw)
                        except Exception as exc:
                            logger.warning(
                                "Failed to update bmc_firmware for asset %s: %s",
                                asset_row.get("asset_tag"), exc,
                            )

        await asyncio.gather(*[_scan(ip) for ip in ips], return_exceptions=True)
        return results

    async def test_connection(self, target: CollectTarget) -> ConnectionResult:
        credentials = target.credentials or {}
        options = target.options or {}
        ips = expand_cidrs(target.endpoint)
        if not ips:
            return ConnectionResult(success=False, message="No IPs resolved from endpoint")
        ip = ips[0]
        import time
        start = time.monotonic()
        try:
            result = await asyncio.to_thread(
                _collect_single_sync, ip, credentials, options
            )
            elapsed = (time.monotonic() - start) * 1000
            if result is not None:
                return ConnectionResult(success=True, latency_ms=elapsed)
            return ConnectionResult(
                success=False,
                message=f"No data returned from {ip}",
                latency_ms=elapsed,
            )
        except Exception as exc:
            elapsed = (time.monotonic() - start) * 1000
            return ConnectionResult(success=False, message=str(exc), latency_ms=elapsed)


# Register the collector
registry.register(IPMICollector())
