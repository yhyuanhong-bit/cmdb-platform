"""SNMP collector — polls network devices via SNMPv1/v2c/v3."""

import asyncio
import ipaddress
import logging
import re
import time
from datetime import datetime, timezone

from pysnmp.hlapi.asyncio import (
    CommunityData,
    ContextData,
    ObjectIdentity,
    ObjectType,
    SnmpEngine,
    UdpTransportTarget,
    UsmUserData,
    get_cmd,
)

from app.models.common import (
    CollectTarget,
    ConnectionResult,
    FieldMapping,
    RawAssetData,
)

logger = logging.getLogger(__name__)

# ──────────────────────────────────────────────
# OID constants
# ──────────────────────────────────────────────
OID_SYS_DESCR = "1.3.6.1.2.1.1.1.0"
OID_SYS_OBJECT_ID = "1.3.6.1.2.1.1.2.0"
OID_SYS_NAME = "1.3.6.1.2.1.1.5.0"
OID_ENT_PHYSICAL_SERIAL = "1.3.6.1.2.1.47.1.1.1.1.11"

VENDOR_OID_PREFIXES: dict[str, str] = {
    "1.3.6.1.4.1.9": "Cisco",
    "1.3.6.1.4.1.11": "HP",
    "1.3.6.1.4.1.674": "Dell",
    "1.3.6.1.4.1.2011": "Huawei",
    "1.3.6.1.4.1.2636": "Juniper",
}

VENDOR_SERIAL_OIDS: dict[str, str] = {
    "HP": "1.3.6.1.4.1.11.2.36.1.1.2.9.0",
    "Dell": "1.3.6.1.4.1.674.10895.3000.1.2.100.8.1.4.1",
    "Huawei": "1.3.6.1.4.1.2011.5.25.188.1.1",
    "Juniper": "1.3.6.1.4.1.2636.3.1.3.0",
}

DEFAULT_CONCURRENCY = 50


# ──────────────────────────────────────────────
# Pure helpers
# ──────────────────────────────────────────────

def expand_cidrs(cidrs: list[str]) -> list[str]:
    """Expand a list of CIDR blocks to individual host IP strings.

    Network and broadcast addresses are excluded so every returned address
    is directly reachable.
    """
    hosts: list[str] = []
    for cidr in cidrs:
        cidr = cidr.strip()
        try:
            network = ipaddress.ip_network(cidr, strict=False)
            # For /32 (single host) include the address itself
            if network.num_addresses == 1:
                hosts.append(str(network.network_address))
            else:
                hosts.extend(str(h) for h in network.hosts())
        except ValueError:
            logger.warning("Invalid CIDR skipped: %s", cidr)
    return hosts


def detect_vendor(sys_object_id: str) -> str:
    """Map a sysObjectID value to a vendor name.

    Tries the longest matching prefix first so more-specific entries win.
    Returns ``"Unknown"`` when no prefix matches.
    """
    oid = sys_object_id.strip()
    # Sort by prefix length descending for longest-match semantics.
    # Require that the match ends at an OID component boundary (a dot or
    # end-of-string) so "1.3.6.1.4.1.9" does not match "1.3.6.1.4.1.99999".
    for prefix in sorted(VENDOR_OID_PREFIXES, key=len, reverse=True):
        if oid == prefix or oid.startswith(prefix + "."):
            return VENDOR_OID_PREFIXES[prefix]
    return "Unknown"


def parse_sysdescr(descr: str) -> dict:
    """Extract model and os_version from a sysDescr string.

    Attempts several heuristic patterns common across vendor descriptions.
    Returns a dict with keys ``model`` and ``os_version``; values are ``None``
    when they cannot be determined.
    """
    result: dict[str, str | None] = {"model": None, "os_version": None}

    if not descr:
        return result

    # Cisco IOS/IOS-XE:  "Cisco IOS Software, ... Version 15.2(4)M3, ..."
    m = re.search(r"Version\s+([\d().A-Za-z][\w.()\-]*)", descr, re.IGNORECASE)
    if m:
        result["os_version"] = m.group(1)

    # Model — look for common keywords
    # Cisco: "cisco WS-C3750X-48P"
    m = re.search(r"(?:cisco|model)[^\w]+([\w\-]+)", descr, re.IGNORECASE)
    if m:
        result["model"] = m.group(1)
    else:
        # Generic: first word-token that looks like a model number (letters+digits+dashes)
        m = re.search(r"\b([A-Z]{1,6}[\-_]?[0-9]{2,}[\w\-]*)\b", descr)
        if m:
            result["model"] = m.group(1)

    return result


# ──────────────────────────────────────────────
# SNMP I/O
# ──────────────────────────────────────────────

def _build_auth_data(credentials: dict | None) -> CommunityData | UsmUserData:
    """Build pysnmp auth object from credential dict."""
    creds = credentials or {}
    version = str(creds.get("version", "2c")).lower()

    if version in ("1", "2c", "v1", "v2c"):
        community = creds.get("community", "public")
        mp_model = 0 if version in ("1", "v1") else 1
        return CommunityData(community, mpModel=mp_model)

    # SNMPv3
    return UsmUserData(
        userName=creds.get("username", ""),
        authKey=creds.get("auth_key"),
        privKey=creds.get("priv_key"),
    )


async def _snmp_get(
    ip: str,
    oids: list[str],
    credentials: dict | None,
    options: dict | None,
) -> dict[str, str]:
    """Perform an SNMP GET for the supplied OIDs against *ip*.

    Returns a mapping of ``oid → value`` (string representation).  Missing or
    error-valued OIDs are silently omitted from the result.
    """
    opts = options or {}
    port = int(opts.get("port", 161))
    timeout = float(opts.get("timeout", 2))
    retries = int(opts.get("retries", 1))

    engine = SnmpEngine()
    auth_data = _build_auth_data(credentials)
    transport = await UdpTransportTarget.create(
        (ip, port),
        timeout=timeout,
        retries=retries,
    )
    context = ContextData()

    var_binds_req = [ObjectType(ObjectIdentity(oid)) for oid in oids]

    error_indication, error_status, _error_index, var_binds = await get_cmd(
        engine,
        auth_data,
        transport,
        context,
        *var_binds_req,
    )

    result: dict[str, str] = {}

    if error_indication:
        logger.debug("SNMP GET error for %s: %s", ip, error_indication)
        return result

    if error_status:
        logger.debug(
            "SNMP GET error status for %s: %s", ip, error_status.prettyPrint()
        )
        return result

    for var_bind in var_binds:
        oid_str = str(var_bind[0])
        value = var_bind[1]
        # Strip the leading dot that pysnmp sometimes adds
        oid_key = oid_str.lstrip(".")
        # Map back to the requested OID if pysnmp returns the full resolved name
        # Try matching any of the requested OIDs as a suffix
        matched_oid = None
        for req_oid in oids:
            clean_req = req_oid.lstrip(".")
            if oid_key == clean_req or oid_key.endswith("." + clean_req) or clean_req.endswith("." + oid_key):
                matched_oid = req_oid
                break
        key = matched_oid or oid_key
        result[key] = value.prettyPrint()

    return result


async def _collect_single(
    ip: str,
    credentials: dict | None,
    options: dict | None,
) -> RawAssetData | None:
    """Probe one IP and, if reachable, collect full asset data.

    Returns a ``RawAssetData`` instance on success or ``None`` if the host
    does not respond to SNMP.
    """
    # Probe with sysDescr — cheap single-OID check
    probe_result = await _snmp_get(
        ip,
        [OID_SYS_DESCR, OID_SYS_OBJECT_ID, OID_SYS_NAME],
        credentials,
        options,
    )
    if not probe_result:
        return None

    sys_descr = probe_result.get(OID_SYS_DESCR, "")
    sys_object_id = probe_result.get(OID_SYS_OBJECT_ID, "")
    sys_name = probe_result.get(OID_SYS_NAME, ip)

    vendor = detect_vendor(sys_object_id)
    parsed = parse_sysdescr(sys_descr)

    # Try standard serial OID first
    serial: str | None = None
    serial_result = await _snmp_get(ip, [OID_ENT_PHYSICAL_SERIAL], credentials, options)
    serial = serial_result.get(OID_ENT_PHYSICAL_SERIAL) or None

    # Vendor-specific fallback
    if not serial and vendor in VENDOR_SERIAL_OIDS:
        fallback_oid = VENDOR_SERIAL_OIDS[vendor]
        fallback_result = await _snmp_get(ip, [fallback_oid], credentials, options)
        serial = fallback_result.get(fallback_oid) or None

    fields: dict[str, str | None] = {
        "hostname": sys_name or None,
        "ip_address": ip,
        "vendor": vendor if vendor != "Unknown" else None,
        "model": parsed.get("model"),
        "os_version": parsed.get("os_version"),
        "serial_number": serial,
    }

    return RawAssetData(
        source="snmp",
        unique_key=f"snmp:{ip}",
        fields=fields,
        attributes={
            "sys_descr": sys_descr,
            "sys_object_id": sys_object_id,
        },
        collected_at=datetime.now(timezone.utc),
    )


# ──────────────────────────────────────────────
# Collector class
# ──────────────────────────────────────────────

class SNMPCollector:
    """Collector that discovers and interrogates network devices via SNMP."""

    name: str = "snmp"
    collect_type: str = "snmp"

    def supported_fields(self) -> list[FieldMapping]:
        return [
            FieldMapping(field_name="hostname", authority=False),
            FieldMapping(field_name="ip_address", authority=True),
            FieldMapping(field_name="vendor", authority=True),
            FieldMapping(field_name="model", authority=True),
            FieldMapping(field_name="os_version", authority=True),
            FieldMapping(field_name="serial_number", authority=True),
        ]

    async def collect(self, target: CollectTarget) -> list[RawAssetData]:
        """Collect SNMP data from all hosts in *target.endpoint*.

        ``target.endpoint`` may be a single IP, a comma-separated list of IPs,
        or CIDR blocks (e.g. ``"10.0.0.0/24,192.168.1.1"``).
        """
        opts = target.options or {}
        concurrency = int(opts.get("concurrency", DEFAULT_CONCURRENCY))
        semaphore = asyncio.Semaphore(concurrency)

        raw_entries = [e.strip() for e in target.endpoint.split(",") if e.strip()]
        # Expand any CIDRs; plain IPs pass through expand_cidrs unchanged (/32)
        ips: list[str] = []
        for entry in raw_entries:
            if "/" in entry:
                ips.extend(expand_cidrs([entry]))
            else:
                ips.append(entry)

        async def bounded_collect(ip: str) -> RawAssetData | None:
            async with semaphore:
                try:
                    return await _collect_single(ip, target.credentials, target.options)
                except Exception as exc:  # noqa: BLE001
                    logger.debug("SNMP collection failed for %s: %s", ip, exc)
                    return None

        results = await asyncio.gather(*[bounded_collect(ip) for ip in ips])
        return [r for r in results if r is not None]

    async def test_connection(self, target: CollectTarget) -> ConnectionResult:
        """Test SNMP reachability for the first IP in *target.endpoint*."""
        ip = target.endpoint.split(",")[0].strip()
        # Resolve CIDR to first host
        if "/" in ip:
            expanded = expand_cidrs([ip])
            if not expanded:
                return ConnectionResult(success=False, message="No hosts in CIDR range")
            ip = expanded[0]

        start = time.monotonic()
        try:
            result = await _snmp_get(
                ip,
                [OID_SYS_DESCR],
                target.credentials,
                target.options,
            )
            latency_ms = (time.monotonic() - start) * 1000
            if result:
                return ConnectionResult(
                    success=True,
                    message=f"SNMP reachable: {result.get(OID_SYS_DESCR, '')[:80]}",
                    latency_ms=latency_ms,
                )
            return ConnectionResult(
                success=False,
                message="No SNMP response",
                latency_ms=latency_ms,
            )
        except Exception as exc:  # noqa: BLE001
            latency_ms = (time.monotonic() - start) * 1000
            return ConnectionResult(success=False, message=str(exc), latency_ms=latency_ms)
