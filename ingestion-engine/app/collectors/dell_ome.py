"""Dell OpenManage Enterprise (OME) collector.

Pulls device inventory and SupportAssist contract data from a Dell OME
appliance. Like OneView, OME is a single endpoint that aggregates every
Dell server it manages — far cheaper than per-device IPMI/iDRAC scraping.

OME 3.5+ is required for the SupportAssist contract endpoint:
  /api/SupportAssistService/Contracts

Older OME (3.4 and earlier) only expose /api/DeviceService/Devices and
warranty fields stay None — PredictiveCapex will mark those rows as
"unknown lifespan", same fallback as OneView.

Endpoint: target.endpoint is the OME base URL
          (e.g. https://ome.example.com).

Credentials in target.credentials:
  - username + password — OME issues a session token via
    POST /api/SessionService/Sessions

Options in target.options:
  - verify_tls (bool, default true)
  - page_size (int, default 200) — Devices/Contracts pagination
  - device_types (list[int], optional) — restrict to e.g. [1000] (servers).
    Default: all device types OME knows about.
"""

from __future__ import annotations

import logging
import time
from datetime import datetime, timezone
from typing import Any

import httpx

from app.collectors.base import Collector, registry
from app.models.common import CollectTarget, ConnectionResult, FieldMapping, RawAssetData

logger = logging.getLogger(__name__)

_DEFAULT_TIMEOUT_S = 30
_DEFAULT_PAGE_SIZE = 200


def _to_iso_date(value: Any) -> str | None:
    """Trim an OME timestamp to YYYY-MM-DD; tolerate None / empty / bad shape."""
    if not value or not isinstance(value, str):
        return None
    return value[:10] if len(value) >= 10 else None


def _map_health(ome_health: Any) -> str:
    """OME reports Status as int (1000=Normal, 2000=Unknown, 3000=Warning,
    4000=Critical, 5000=NoStatus). Map to CMDB status enum."""
    if ome_health is None:
        return "operational"
    try:
        n = int(ome_health)
    except (TypeError, ValueError):
        return "operational"
    if n in (1000, 2000):
        return "operational"
    if n in (3000, 4000):
        return "maintenance"
    if n == 5000:
        return "decommissioned"
    return "operational"


def _device_to_asset(row: dict, contracts_by_tag: dict[str, dict]) -> RawAssetData:
    """Map one /api/DeviceService/Devices row to a RawAssetData.

    Joins SupportAssist contract data on service_tag.
    """
    service_tag = row.get("DeviceServiceTag") or row.get("ServiceTag") or ""
    model = row.get("Model") or row.get("DeviceModel") or ""
    name = row.get("DeviceName") or row.get("Identifier") or service_tag

    contract = contracts_by_tag.get(service_tag, {})
    warranty_start = _to_iso_date(contract.get("StartDate"))
    warranty_end = _to_iso_date(contract.get("EndDate"))
    warranty_vendor = contract.get("DeviceType") or "Dell ProSupport"
    warranty_contract = contract.get("ContractCode") or contract.get("ContractId")

    fields: dict[str, str | None] = {
        "vendor": "Dell",
        "model": model,
        "serial_number": service_tag or None,
        "name": name,
        "type": "server",
        "status": _map_health(row.get("Status")),
        "warranty_start": warranty_start,
        "warranty_end": warranty_end,
        "warranty_vendor": warranty_vendor,
        "warranty_contract": str(warranty_contract) if warranty_contract is not None else None,
    }
    unique_key = service_tag or row.get("Identifier") or name

    return RawAssetData(
        source="dell_ome",
        unique_key=unique_key,
        fields=fields,
        attributes={
            "ome_id": row.get("Id"),
            "ome_status": row.get("Status"),
            "ome_managed_state": row.get("ManagedState"),
            "ome_device_type": row.get("Type"),
        },
        collected_at=datetime.now(timezone.utc),
    )


async def _login(client: httpx.AsyncClient, base_url: str, creds: dict) -> str:
    """POST /api/SessionService/Sessions, return X-Auth-Token header value."""
    resp = await client.post(
        f"{base_url.rstrip('/')}/api/SessionService/Sessions",
        json={
            "UserName": creds["username"],
            "Password": creds["password"],
            "SessionType": "API",
        },
        timeout=_DEFAULT_TIMEOUT_S,
    )
    resp.raise_for_status()
    # OME returns the auth token in the X-Auth-Token response header
    return resp.headers.get("X-Auth-Token", "")


async def _fetch_paginated(
    client: httpx.AsyncClient,
    url: str,
    headers: dict,
    page_size: int,
) -> list[dict]:
    """Walk OME's `@odata.nextLink` pagination until exhausted."""
    out: list[dict] = []
    next_url: str | None = url
    while next_url:
        resp = await client.get(next_url, headers=headers, params={"$top": page_size})
        resp.raise_for_status()
        body = resp.json()
        out.extend(body.get("value") or [])
        next_url = body.get("@odata.nextLink")
        # Some OME versions return a relative link — anchor against base
        if next_url and next_url.startswith("/"):
            base = url.rsplit("/api/", 1)[0]
            next_url = f"{base}{next_url}"
    return out


class DellOMECollector:
    """Collect inventory + warranty from a Dell OpenManage Enterprise.

    Implements the Collector protocol from app.collectors.base.
    """

    name = "dell_ome"
    collect_type = "dell_ome"

    def supported_fields(self) -> list[FieldMapping]:
        return [
            FieldMapping(field_name="vendor", authority=True),
            FieldMapping(field_name="model", authority=True),
            FieldMapping(field_name="serial_number", authority=True),
            FieldMapping(field_name="name"),
            FieldMapping(field_name="status"),
            # OME with SupportAssist is the authoritative source for Dell
            # warranty data — same precedence rule as OneView for HPE.
            FieldMapping(field_name="warranty_start", authority=True),
            FieldMapping(field_name="warranty_end", authority=True),
            FieldMapping(field_name="warranty_vendor", authority=True),
            FieldMapping(field_name="warranty_contract", authority=True),
        ]

    async def collect(self, target: CollectTarget) -> list[RawAssetData]:
        creds = target.credentials or {}
        opts = target.options or {}
        verify_tls = bool(opts.get("verify_tls", True))
        page_size = int(opts.get("page_size", _DEFAULT_PAGE_SIZE))

        base_url = target.endpoint.rstrip("/")
        async with httpx.AsyncClient(verify=verify_tls, timeout=_DEFAULT_TIMEOUT_S) as client:
            token = await _login(client, base_url, creds)
            if not token:
                logger.warning("dell_ome: login returned empty token")
                return []
            headers = {"X-Auth-Token": token, "Accept": "application/json"}

            devices = await _fetch_paginated(
                client, f"{base_url}/api/DeviceService/Devices", headers, page_size,
            )

            # Pull SupportAssist contracts. Fail-soft: if endpoint returns
            # 404 (OME < 3.5) or 403 (no SupportAssist license), log and
            # continue with no warranty data — devices still surface,
            # just without warranty fields.
            contracts_by_tag: dict[str, dict] = {}
            try:
                contracts = await _fetch_paginated(
                    client, f"{base_url}/api/SupportAssistService/Contracts",
                    headers, page_size,
                )
                for c in contracts:
                    tag = c.get("DeviceServiceTag") or c.get("ServiceTag")
                    if tag:
                        contracts_by_tag[tag] = c
            except httpx.HTTPStatusError as exc:
                if exc.response.status_code in (403, 404):
                    logger.info(
                        "dell_ome: SupportAssist contracts unavailable (HTTP %d) — "
                        "warranty data will be empty", exc.response.status_code,
                    )
                else:
                    raise

        results = [_device_to_asset(d, contracts_by_tag) for d in devices]
        logger.info(
            "dell_ome: collected %d devices, %d with contract", len(results), len(contracts_by_tag),
        )
        return results

    async def test_connection(self, target: CollectTarget) -> ConnectionResult:
        creds = target.credentials or {}
        opts = target.options or {}
        verify_tls = bool(opts.get("verify_tls", True))
        base_url = target.endpoint.rstrip("/")

        t0 = time.monotonic()
        try:
            async with httpx.AsyncClient(verify=verify_tls, timeout=_DEFAULT_TIMEOUT_S) as client:
                await _login(client, base_url, creds)
            latency = (time.monotonic() - t0) * 1000
            return ConnectionResult(success=True, latency_ms=latency)
        except httpx.HTTPStatusError as exc:
            return ConnectionResult(
                success=False,
                message=f"OME HTTP {exc.response.status_code}: {exc.response.text[:200]}",
                latency_ms=(time.monotonic() - t0) * 1000,
            )
        except httpx.RequestError as exc:
            return ConnectionResult(
                success=False,
                message=f"OME unreachable: {exc}",
                latency_ms=(time.monotonic() - t0) * 1000,
            )


registry.register(DellOMECollector())
