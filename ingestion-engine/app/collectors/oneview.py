"""HPE OneView collector.

Pulls server hardware inventory from a HPE OneView appliance instead of
scanning per-IP. One OneView endpoint returns *every* HPE server it
manages in a single call, which is significantly cheaper than walking
SNMP across the same fleet.

Lifecycle fields (warranty / EOL / contract) come from the *Remote
Support* feature of OneView 5.x+. Older appliances (pre-5.0) only
expose hardware inventory; the warranty fields will be None and
PredictiveCapex will surface those rows as 'unknown lifespan'.

Endpoint: target.endpoint is the OneView base URL
          (e.g. https://oneview.example.com).

Credentials in target.credentials:
  - username (string) and password (string) — OneView creates a
    session token on POST /rest/login-sessions
  - or api_token (string) — pre-issued OneView API key, skips login

Options in target.options:
  - api_version (int, default 4000) — X-API-Version header. OneView 5.x
    accepts up to 4600; higher values get a 412 from older appliances.
  - verify_tls (bool, default true) — set false in lab environments
    where OneView ships a self-signed cert. Production must keep it on.
  - page_size (int, default 200) — server-hardware list pagination.
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

# OneView returns ISO 8601 timestamps. We parse them into asset DATE
# fields (warranty_end, eol_date, etc) which the cmdb-core ALTER from
# 000036 stores as DATE. Trim time portion before sending upstream.
_DEFAULT_API_VERSION = 4000
_DEFAULT_TIMEOUT_S = 30
_DEFAULT_PAGE_SIZE = 200


def _login(client: httpx.AsyncClient, base_url: str, creds: dict, api_version: int) -> str:
    """POST /rest/login-sessions, return the session token."""
    resp = client.post(
        f"{base_url.rstrip('/')}/rest/login-sessions",
        headers={"X-API-Version": str(api_version), "Content-Type": "application/json"},
        json={"userName": creds["username"], "password": creds["password"]},
        timeout=_DEFAULT_TIMEOUT_S,
    )
    resp.raise_for_status()
    body = resp.json()
    return body.get("sessionID") or body.get("session_id") or ""


async def _login_async(client: httpx.AsyncClient, base_url: str, creds: dict, api_version: int) -> str:
    resp = await client.post(
        f"{base_url.rstrip('/')}/rest/login-sessions",
        headers={"X-API-Version": str(api_version), "Content-Type": "application/json"},
        json={"userName": creds["username"], "password": creds["password"]},
        timeout=_DEFAULT_TIMEOUT_S,
    )
    resp.raise_for_status()
    body = resp.json()
    return body.get("sessionID") or body.get("session_id") or ""


def _to_iso_date(value: Any) -> str | None:
    """Trim a OneView timestamp to YYYY-MM-DD; tolerate None / empty / bad shape."""
    if not value or not isinstance(value, str):
        return None
    # OneView returns "2026-04-15T00:00:00.000Z" — slice the date prefix
    return value[:10] if len(value) >= 10 else None


def _row_to_asset(row: dict) -> RawAssetData:
    """Map one /rest/server-hardware row to a RawAssetData for the pipeline."""
    serial = row.get("serialNumber") or row.get("serial_number") or ""
    model = row.get("model") or ""
    name = row.get("name") or row.get("uri") or serial

    # Extract optional lifecycle fields from the embedded supportContract
    # block (OneView 5.x+ with Remote Support enabled). Older payloads
    # simply omit the key — None propagates downstream and PredictiveCapex
    # treats those rows as "unknown".
    contract = row.get("supportContract") or {}
    warranty_start = _to_iso_date(contract.get("startDate"))
    warranty_end = _to_iso_date(contract.get("endDate"))
    warranty_vendor = contract.get("contractType") or "HPE"
    warranty_contract = contract.get("contractNumber")

    fields: dict[str, str | None] = {
        "vendor": "HPE",
        "model": model,
        "serial_number": serial or None,
        "name": name,
        "type": "server",
        "status": _map_status(row.get("status")),
        "warranty_start": warranty_start,
        "warranty_end": warranty_end,
        "warranty_vendor": warranty_vendor,
        "warranty_contract": warranty_contract,
    }
    # Derive a unique_key. Prefer serial; fall back to OneView URI so two
    # racks with blank serial don't collide on "" and dedupe wrongly.
    unique_key = serial or row.get("uri") or name

    return RawAssetData(
        source="oneview",
        unique_key=unique_key,
        fields=fields,
        attributes={
            "oneview_uri": row.get("uri"),
            "oneview_status": row.get("status"),
            "rom_version": row.get("romVersion"),
            "ilo_firmware": row.get("mpFirmwareVersion"),
        },
        collected_at=datetime.now(timezone.utc),
    )


def _map_status(oneview_status: str | None) -> str:
    """OneView reports OK / Warning / Critical / Disabled / Unknown.

    Map onto the CMDB status enum so the downstream pipeline accepts it
    without churn.
    """
    if not oneview_status:
        return "operational"
    s = oneview_status.lower()
    if s in ("ok", "normal"):
        return "operational"
    if s in ("warning", "critical"):
        return "maintenance"
    if s == "disabled":
        return "decommissioned"
    return "operational"


class OneViewCollector:
    """Collect inventory + warranty from a HPE OneView appliance.

    Implements the Collector protocol from app.collectors.base.
    """

    name = "oneview"
    collect_type = "oneview"

    def supported_fields(self) -> list[FieldMapping]:
        return [
            FieldMapping(field_name="vendor", authority=True),
            FieldMapping(field_name="model", authority=True),
            FieldMapping(field_name="serial_number", authority=True),
            FieldMapping(field_name="name"),
            FieldMapping(field_name="status"),
            # OneView is the *authoritative* source for lifecycle data
            # because it pulls directly from the HPE Support contract.
            FieldMapping(field_name="warranty_start", authority=True),
            FieldMapping(field_name="warranty_end", authority=True),
            FieldMapping(field_name="warranty_vendor", authority=True),
            FieldMapping(field_name="warranty_contract", authority=True),
        ]

    async def collect(self, target: CollectTarget) -> list[RawAssetData]:
        creds = target.credentials or {}
        opts = target.options or {}
        api_version = int(opts.get("api_version", _DEFAULT_API_VERSION))
        verify_tls = bool(opts.get("verify_tls", True))
        page_size = int(opts.get("page_size", _DEFAULT_PAGE_SIZE))

        base_url = target.endpoint.rstrip("/")
        async with httpx.AsyncClient(verify=verify_tls, timeout=_DEFAULT_TIMEOUT_S) as client:
            token = creds.get("api_token") or await _login_async(client, base_url, creds, api_version)
            if not token:
                logger.warning("oneview: login returned empty session id")
                return []

            headers = {
                "Auth": token,
                "X-API-Version": str(api_version),
                "Accept": "application/json",
            }
            results: list[RawAssetData] = []
            start = 0
            while True:
                resp = await client.get(
                    f"{base_url}/rest/server-hardware",
                    headers=headers,
                    params={"start": start, "count": page_size},
                )
                resp.raise_for_status()
                body = resp.json()
                members = body.get("members") or []
                for row in members:
                    try:
                        results.append(_row_to_asset(row))
                    except Exception as exc:  # pragma: no cover - defensive
                        logger.warning("oneview: row mapping failed: %s", exc)
                # Pagination: OneView returns 'nextPageUri' when more rows
                # exist; absence means we're done. count<page_size is also a
                # safe termination signal for older appliances.
                if not body.get("nextPageUri") or len(members) < page_size:
                    break
                start += page_size

        logger.info("oneview: collected %d server-hardware rows", len(results))
        return results

    async def test_connection(self, target: CollectTarget) -> ConnectionResult:
        creds = target.credentials or {}
        opts = target.options or {}
        api_version = int(opts.get("api_version", _DEFAULT_API_VERSION))
        verify_tls = bool(opts.get("verify_tls", True))
        base_url = target.endpoint.rstrip("/")

        t0 = time.monotonic()
        try:
            async with httpx.AsyncClient(verify=verify_tls, timeout=_DEFAULT_TIMEOUT_S) as client:
                # Either api_token or username+password must work
                if creds.get("api_token"):
                    resp = await client.get(
                        f"{base_url}/rest/version",
                        headers={"X-API-Version": str(api_version)},
                    )
                    resp.raise_for_status()
                else:
                    await _login_async(client, base_url, creds, api_version)
            latency = (time.monotonic() - t0) * 1000
            return ConnectionResult(success=True, latency_ms=latency)
        except httpx.HTTPStatusError as exc:
            return ConnectionResult(
                success=False,
                message=f"OneView HTTP {exc.response.status_code}: {exc.response.text[:200]}",
                latency_ms=(time.monotonic() - t0) * 1000,
            )
        except httpx.RequestError as exc:
            return ConnectionResult(
                success=False,
                message=f"OneView unreachable: {exc}",
                latency_ms=(time.monotonic() - t0) * 1000,
            )


registry.register(OneViewCollector())
