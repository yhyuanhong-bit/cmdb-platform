"""Unit tests for the HPE OneView collector.

Mocks httpx.AsyncClient so we exercise the row-mapping + login + paging
logic without a live OneView appliance. Live integration is verified
manually against a test OneView instance before each release.
"""

from __future__ import annotations

from unittest.mock import AsyncMock, patch

import pytest

from app.collectors.oneview import OneViewCollector, _map_status, _row_to_asset, _to_iso_date
from app.models.common import CollectTarget


class TestRowMapping:
    """Pure-fn tests — no I/O."""

    def test_full_row_with_warranty(self):
        row = {
            "name": "DL360-Gen10-01",
            "uri": "/rest/server-hardware/abc",
            "serialNumber": "MXQ12345AB",
            "model": "ProLiant DL360 Gen10",
            "status": "OK",
            "romVersion": "U32 v2.78",
            "mpFirmwareVersion": "iLO 5 v2.81",
            "supportContract": {
                "startDate": "2024-01-15T00:00:00.000Z",
                "endDate": "2027-01-15T00:00:00.000Z",
                "contractType": "HPE Pointnext Tech Care",
                "contractNumber": "CTR-2024-001",
            },
        }
        asset = _row_to_asset(row)
        assert asset.source == "oneview"
        assert asset.unique_key == "MXQ12345AB"
        assert asset.fields["vendor"] == "HPE"
        assert asset.fields["model"] == "ProLiant DL360 Gen10"
        assert asset.fields["serial_number"] == "MXQ12345AB"
        assert asset.fields["warranty_start"] == "2024-01-15"
        assert asset.fields["warranty_end"] == "2027-01-15"
        assert asset.fields["warranty_vendor"] == "HPE Pointnext Tech Care"
        assert asset.fields["warranty_contract"] == "CTR-2024-001"
        assert asset.fields["status"] == "operational"
        assert asset.attributes["oneview_uri"] == "/rest/server-hardware/abc"
        assert asset.attributes["rom_version"] == "U32 v2.78"

    def test_row_without_support_contract(self):
        """OneView pre-5.0 or appliances without Remote Support — warranty fields stay None."""
        row = {
            "name": "old-server",
            "uri": "/rest/server-hardware/old",
            "serialNumber": "OLD-001",
            "model": "ProLiant DL380 Gen9",
            "status": "OK",
        }
        asset = _row_to_asset(row)
        assert asset.fields["warranty_start"] is None
        assert asset.fields["warranty_end"] is None
        assert asset.fields["warranty_contract"] is None
        # Vendor falls back to "HPE" even without contract block
        assert asset.fields["warranty_vendor"] == "HPE"

    def test_row_with_blank_serial_falls_back_to_uri(self):
        """Blank serial must NOT collapse multiple rows to unique_key=''."""
        row = {
            "name": "rack-1",
            "uri": "/rest/server-hardware/12345",
            "serialNumber": "",
            "model": "Synergy 480 Gen10",
            "status": "OK",
        }
        asset = _row_to_asset(row)
        assert asset.unique_key == "/rest/server-hardware/12345"
        assert asset.fields["serial_number"] is None

    @pytest.mark.parametrize(
        "ov_status, expected",
        [
            ("OK", "operational"),
            ("Normal", "operational"),
            ("Warning", "maintenance"),
            ("Critical", "maintenance"),
            ("Disabled", "decommissioned"),
            ("Unknown", "operational"),
            (None, "operational"),
            ("", "operational"),
        ],
    )
    def test_status_mapping(self, ov_status, expected):
        assert _map_status(ov_status) == expected

    @pytest.mark.parametrize(
        "raw, expected",
        [
            ("2026-04-15T00:00:00.000Z", "2026-04-15"),
            ("2026-04-15", "2026-04-15"),
            ("", None),
            (None, None),
            (123, None),  # non-string → None
        ],
    )
    def test_iso_date_trimming(self, raw, expected):
        assert _to_iso_date(raw) == expected


class TestSupportedFields:
    def test_authoritative_for_warranty_lifecycle(self):
        """OneView is the *source of truth* for HPE warranty data —
        the FieldMapping list must mark those fields as authority=True
        so the merge layer doesn't overwrite OneView with a stale CSV."""
        collector = OneViewCollector()
        names_to_authority = {fm.field_name: fm.authority for fm in collector.supported_fields()}
        assert names_to_authority["warranty_start"] is True
        assert names_to_authority["warranty_end"] is True
        assert names_to_authority["warranty_vendor"] is True
        assert names_to_authority["warranty_contract"] is True
        assert names_to_authority["serial_number"] is True


class TestCollectIntegration:
    """End-to-end mock — login + list page + map → RawAssetData."""

    @pytest.mark.asyncio
    async def test_collect_paginates_and_stops(self):
        collector = OneViewCollector()
        target = CollectTarget(
            endpoint="https://oneview.example.com",
            credentials={"username": "admin", "password": "secret"},
            options={"api_version": 4000, "verify_tls": False, "page_size": 2},
        )

        page1 = {
            "members": [
                {
                    "name": "a", "uri": "/rest/server-hardware/a",
                    "serialNumber": "AAA1", "model": "DL360 Gen10", "status": "OK",
                },
                {
                    "name": "b", "uri": "/rest/server-hardware/b",
                    "serialNumber": "BBB2", "model": "DL360 Gen10", "status": "Warning",
                },
            ],
            "nextPageUri": "/rest/server-hardware?start=2&count=2",
        }
        page2 = {
            "members": [
                {
                    "name": "c", "uri": "/rest/server-hardware/c",
                    "serialNumber": "CCC3", "model": "DL380 Gen10", "status": "OK",
                },
            ],
            # No nextPageUri → terminate
        }

        # Mock login response + two GET pages
        login_response = AsyncMock(status_code=200)
        login_response.json = lambda: {"sessionID": "tok-xyz"}
        login_response.raise_for_status = lambda: None

        get_responses = [
            _mk_resp(page1),
            _mk_resp(page2),
        ]

        with patch("app.collectors.oneview.httpx.AsyncClient") as mock_cls:
            client = mock_cls.return_value.__aenter__.return_value
            client.post = AsyncMock(return_value=login_response)
            client.get = AsyncMock(side_effect=get_responses)

            results = await collector.collect(target)

        assert len(results) == 3
        assert results[0].fields["serial_number"] == "AAA1"
        assert results[1].fields["status"] == "maintenance"
        assert results[2].fields["serial_number"] == "CCC3"

    @pytest.mark.asyncio
    async def test_collect_uses_api_token_skips_login(self):
        """When api_token is supplied, no POST /rest/login-sessions roundtrip."""
        collector = OneViewCollector()
        target = CollectTarget(
            endpoint="https://oneview.example.com",
            credentials={"api_token": "pre-issued-token"},
            options={"page_size": 100, "verify_tls": False},
        )

        with patch("app.collectors.oneview.httpx.AsyncClient") as mock_cls:
            client = mock_cls.return_value.__aenter__.return_value
            client.post = AsyncMock()  # MUST not be called
            client.get = AsyncMock(return_value=_mk_resp({"members": [], "nextPageUri": None}))

            await collector.collect(target)

            # The login endpoint must NOT have been hit
            assert client.post.call_count == 0


def _mk_resp(payload: dict):
    """Helper: construct a mock httpx.Response yielding `payload` from .json()."""
    resp = AsyncMock()
    resp.status_code = 200
    resp.json = lambda: payload
    resp.raise_for_status = lambda: None
    return resp
