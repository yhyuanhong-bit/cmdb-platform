"""SSH collector — connects to remote hosts and collects hardware/OS metadata."""

from __future__ import annotations

import asyncio
import logging
import shlex
from datetime import datetime, timezone
from typing import Optional

import asyncssh

from app.collectors.base import Collector, registry
from app.models.common import CollectTarget, ConnectionResult, FieldMapping, RawAssetData

logger = logging.getLogger(__name__)

# Values that dmidecode reports when the field is not actually populated.
_DMIDECODE_JUNK = frozenset(
    [
        "Not Specified",
        "To Be Filled By O.E.M.",
        "To Be Filled By O.E.M",
        "Default string",
        "Default String",
        "System Product Name",
        "System Manufacturer",
        "Unknown",
        "None",
        "",
    ]
)


def _clean(value: str) -> Optional[str]:
    """Strip whitespace and return None for known-junk dmidecode values."""
    v = value.strip()
    return None if v in _DMIDECODE_JUNK else v


def parse_os_release(content: str) -> dict:
    """Parse /etc/os-release and return a dict with 'os_type' and 'os_version'.

    Returns empty-string values when the file is absent or the keys are missing.
    """
    result: dict[str, str] = {"os_type": "", "os_version": ""}
    for line in content.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        if "=" not in line:
            continue
        key, _, raw_val = line.partition("=")
        # Values may be quoted
        val = raw_val.strip().strip('"').strip("'")
        if key == "ID":
            result["os_type"] = val
        elif key == "VERSION_ID":
            result["os_version"] = val
    return result


# Commands to execute on the remote host, keyed by logical field name.
_COMMANDS: list[tuple[str, str]] = [
    ("hostname", "hostname"),
    (
        "serial_number",
        "dmidecode -s system-serial-number 2>/dev/null || echo ''",
    ),
    (
        "vendor",
        "dmidecode -s system-manufacturer 2>/dev/null || echo ''",
    ),
    (
        "model",
        "dmidecode -s system-product-name 2>/dev/null || echo ''",
    ),
    (
        "os_release",
        "cat /etc/os-release 2>/dev/null || echo ''",
    ),
    (
        "cpu_cores",
        "nproc 2>/dev/null || echo ''",
    ),
    (
        "memory_mb",
        "free -m 2>/dev/null | awk '/Mem:/{print $2}' || echo ''",
    ),
    (
        "disk_gb",
        "lsblk -dbn -o SIZE 2>/dev/null | awk '{s+=$1}END{print int(s/1073741824)}' || echo ''",
    ),
    (
        "ip_address",
        "ip -4 addr show 2>/dev/null | grep 'inet ' | grep -v '127.0.0.1' | awk '{print $2}' | head -1 || echo ''",
    ),
]


async def _connect_and_collect(
    ip: str,
    credentials: dict,
    options: dict,
) -> Optional[RawAssetData]:
    """Open an SSH connection, run commands, and return a RawAssetData.

    Returns None on connection / auth failure.
    """
    username: str = credentials.get("username", "root")
    password: Optional[str] = credentials.get("password")
    private_key: Optional[str] = credentials.get("private_key")
    port: int = int(credentials.get("port", options.get("port", 22)))

    connect_kwargs: dict = {
        "host": ip,
        "port": port,
        "username": username,
        "known_hosts": None,
    }
    if private_key:
        connect_kwargs["client_keys"] = [asyncssh.import_private_key(private_key)]
    elif password:
        connect_kwargs["password"] = password

    try:
        async with asyncssh.connect(**connect_kwargs) as conn:
            raw_results: dict[str, str] = {}
            for field_key, cmd in _COMMANDS:
                result = await conn.run(cmd, check=False)
                raw_results[field_key] = (result.stdout or "").strip()

            # Detect virtual vs physical
            for virt_key, virt_cmd in [
                ("product_name", "cat /sys/class/dmi/id/product_name 2>/dev/null"),
                ("sys_vendor", "cat /sys/class/dmi/id/sys_vendor 2>/dev/null"),
                ("hypervisor", "systemd-detect-virt 2>/dev/null"),
            ]:
                result = await conn.run(virt_cmd, check=False)
                raw_results[virt_key] = (result.stdout or "").strip()

    except (asyncssh.Error, OSError) as exc:
        logger.warning("SSH collection failed for %s: %s", ip, exc)
        return None

    # Build the fields dict
    fields: dict[str, Optional[str]] = {}

    fields["hostname"] = raw_results.get("hostname") or None

    fields["serial_number"] = _clean(raw_results.get("serial_number", ""))
    fields["vendor"] = _clean(raw_results.get("vendor", ""))
    fields["model"] = _clean(raw_results.get("model", ""))

    os_info = parse_os_release(raw_results.get("os_release", ""))
    fields["os_type"] = os_info["os_type"] or None
    fields["os_version"] = os_info["os_version"] or None

    for numeric_field in ("cpu_cores", "memory_mb", "disk_gb"):
        val = raw_results.get(numeric_field, "").strip()
        fields[numeric_field] = val if val.lstrip("-").isdigit() else None

    # Strip CIDR prefix from ip_address (e.g. "192.168.1.5/24" → "192.168.1.5")
    raw_ip = raw_results.get("ip_address", "").strip()
    fields["ip_address"] = raw_ip.split("/")[0] if raw_ip else None

    # Determine virtual vs physical
    product_name = raw_results.get("product_name", "")
    sys_vendor = raw_results.get("sys_vendor", "")
    hypervisor = raw_results.get("hypervisor", "")
    vm_indicators = ["vmware", "kvm", "qemu", "virtualbox", "hyper-v", "xen", "parallels", "bhyve"]
    combined = f"{product_name} {sys_vendor} {hypervisor}".lower()
    if any(v in combined for v in vm_indicators):
        fields["sub_type"] = "virtual"
    elif hypervisor and hypervisor != "none":
        fields["sub_type"] = "virtual"
    else:
        fields["sub_type"] = "physical"

    unique_key = fields.get("serial_number") or fields.get("hostname") or ip

    return RawAssetData(
        source="ssh",
        unique_key=unique_key,
        fields={k: v for k, v in fields.items()},
        collected_at=datetime.now(timezone.utc),
    )


def _expand_targets(endpoint: str) -> list[str]:
    """Expand a CIDR range or return a single IP/hostname list.

    Tries to import expand_cidrs from app.collectors.snmp (Task 7).
    Falls back to treating the endpoint as a single target if the import
    is not yet available.
    """
    try:
        from app.collectors.snmp import expand_cidrs  # type: ignore[import]

        return expand_cidrs(endpoint)
    except ImportError:
        pass

    # Minimal built-in fallback: handle simple CIDR /N notation
    import ipaddress

    try:
        network = ipaddress.ip_network(endpoint, strict=False)
        return [str(host) for host in network.hosts()] or [str(network.network_address)]
    except ValueError:
        return [endpoint]


class SSHCollector:
    """Collector that gathers hardware and OS metadata via SSH."""

    name: str = "ssh"
    collect_type: str = "ssh"

    def __init__(self, concurrency: int = 10) -> None:
        self._semaphore = asyncio.Semaphore(concurrency)

    # ------------------------------------------------------------------
    # Collector protocol
    # ------------------------------------------------------------------

    def supported_fields(self) -> list[FieldMapping]:
        return [
            FieldMapping(field_name="hostname"),
            FieldMapping(field_name="serial_number"),
            FieldMapping(field_name="vendor"),
            FieldMapping(field_name="model"),
            FieldMapping(field_name="os_type"),
            FieldMapping(field_name="os_version"),
            FieldMapping(field_name="cpu_cores"),
            FieldMapping(field_name="memory_mb"),
            FieldMapping(field_name="disk_gb"),
            FieldMapping(field_name="ip_address"),
            FieldMapping(field_name="sub_type"),
        ]

    async def collect(self, target: CollectTarget) -> list[RawAssetData]:
        credentials = target.credentials or {}
        options = target.options or {}
        ips = _expand_targets(target.endpoint)

        async def _guarded(ip: str) -> Optional[RawAssetData]:
            async with self._semaphore:
                return await _connect_and_collect(ip, credentials, options)

        results = await asyncio.gather(*[_guarded(ip) for ip in ips])
        return [r for r in results if r is not None]

    async def test_connection(self, target: CollectTarget) -> ConnectionResult:
        import time

        credentials = target.credentials or {}
        options = target.options or {}
        ips = _expand_targets(target.endpoint)
        if not ips:
            return ConnectionResult(success=False, message="No targets resolved")

        ip = ips[0]
        t0 = time.monotonic()
        result = await _connect_and_collect(ip, credentials, options)
        latency = (time.monotonic() - t0) * 1000

        if result is not None:
            return ConnectionResult(success=True, latency_ms=latency)
        return ConnectionResult(
            success=False,
            message=f"Could not connect to {ip} via SSH",
            latency_ms=latency,
        )


# Register the collector instance
registry.register(SSHCollector())
