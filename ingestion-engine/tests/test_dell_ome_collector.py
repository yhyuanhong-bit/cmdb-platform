"""Unit tests for the Dell OpenManage Enterprise (OME) collector.

Same approach as test_oneview_collector.py — mock httpx.AsyncClient,
exercise mapping + paginate + contract-join + fail-soft on 404.
"""

from __future__ import annotations

from unittest.mock import AsyncMock, patch

import httpx
import pytest

from app.collectors.dell_ome import (
    DellOMECollector,
    _device_to_asset,
    _map_health,
    _to_iso_date,
)
from app.models.common import CollectTarget


class TestDeviceMapping:
    def test_full_device_with_contract(self):
        device = {
            "Id": 12345,
            "Identifier": "Dell-R740-01",
            "DeviceServiceTag": "ABC1234",
            "Model": "PowerEdge R740",
            "DeviceName": "web01.corp",
            "Status": 1000,
            "ManagedState": 4000,
            "Type": 1000,
        }
        contract = {
            "StartDate": "2024-02-01T00:00:00Z",
            "EndDate": "2027-02-01T00:00:00Z",
            "DeviceType": "ProSupport Plus",
            "ContractCode": "SVC-2024-DELL-001",
        }
        asset = _device_to_asset(device, {"ABC1234": contract})

        assert asset.source == "dell_ome"
        assert asset.unique_key == "ABC1234"
        assert asset.fields["vendor"] == "Dell"
        assert asset.fields["model"] == "PowerEdge R740"
        assert asset.fields["serial_number"] == "ABC1234"
        assert asset.fields["warranty_start"] == "2024-02-01"
        assert asset.fields["warranty_end"] == "2027-02-01"
        assert asset.fields["warranty_vendor"] == "ProSupport Plus"
        assert asset.fields["warranty_contract"] == "SVC-2024-DELL-001"
        assert asset.fields["status"] == "operational"

    def test_device_without_contract_match(self):
        """Device exists in OME but SupportAssist has no row for its tag —
        warranty fields stay None and vendor falls back to default."""
        device = {
            "DeviceServiceTag": "ZZZ9999",
            "Model": "PowerEdge R650",
            "DeviceName": "lone-server",
            "Status": 1000,
        }
        asset = _device_to_asset(device, {})  # empty contracts map
        assert asset.fields["warranty_start"] is None
        assert asset.fields["warranty_end"] is None
        assert asset.fields["warranty_vendor"] == "Dell ProSupport"  # default
        assert asset.fields["warranty_contract"] is None

    def test_device_blank_service_tag(self):
        device = {
            "Identifier": "ome-virtual-1",
            "DeviceServiceTag": "",
            "Model": "VxRail",
            "DeviceName": "vx1",
            "Status": 1000,
        }
        asset = _device_to_asset(device, {})
        assert asset.unique_key == "ome-virtual-1"
        assert asset.fields["serial_number"] is None

    @pytest.mark.parametrize(
        "ome_status, expected",
        [
            (1000, "operational"),
            (2000, "operational"),
            (3000, "maintenance"),
            (4000, "maintenance"),
            (5000, "decommissioned"),
            (None, "operational"),
            ("not-a-number", "operational"),
            (9999, "operational"),
        ],
    )
    def test_health_mapping(self, ome_status, expected):
        assert _map_health(ome_status) == expected

    @pytest.mark.parametrize(
        "raw, expected",
        [
            ("2024-02-01T00:00:00Z", "2024-02-01"),
            ("2024-02-01", "2024-02-01"),
            ("", None),
            (None, None),
            (42, None),
        ],
    )
    def test_iso_trim(self, raw, expected):
        assert _to_iso_date(raw) == expected

    def test_contract_id_coerced_to_string(self):
        """OME ContractId is sometimes returned as int — must serialise to
        str so the downstream `text` column accepts it."""
        device = {
            "DeviceServiceTag": "INT01",
            "Model": "PowerEdge",
            "DeviceName": "x",
            "Status": 1000,
        }
        contract = {"ContractId": 778899}  # int, not string
        asset = _device_to_asset(device, {"INT01": contract})
        assert asset.fields["warranty_contract"] == "778899"


class TestSupportedFields:
    def test_authority_flags(self):
        names_to_authority = {
            fm.field_name: fm.authority for fm in DellOMECollector().supported_fields()
        }
        assert names_to_authority["serial_number"] is True
        assert names_to_authority["warranty_end"] is True
        assert names_to_authority["warranty_contract"] is True


class TestCollectIntegration:
    @pytest.mark.asyncio
    async def test_collect_joins_devices_and_contracts(self):
        collector = DellOMECollector()
        target = CollectTarget(
            endpoint="https://ome.example.com",
            credentials={"username": "admin", "password": "secret"},
            options={"verify_tls": False, "page_size": 100},
        )

        login_resp = AsyncMock(status_code=200)
        login_resp.headers = {"X-Auth-Token": "tok-ome-xyz"}
        login_resp.raise_for_status = lambda: None
        login_resp.json = lambda: {}

        devices_page = _mk_resp({
            "value": [
                {"DeviceServiceTag": "AAA1", "Model": "R740", "DeviceName": "a", "Status": 1000},
                {"DeviceServiceTag": "BBB2", "Model": "R650", "DeviceName": "b", "Status": 3000},
            ],
        })
        contracts_page = _mk_resp({
            "value": [
                {
                    "DeviceServiceTag": "AAA1", "StartDate": "2024-01-01", "EndDate": "2027-01-01",
                    "DeviceType": "ProSupport", "ContractCode": "C-AAA",
                },
                # BBB2 has no contract — exercises the unmatched device path
            ],
        })

        with patch("app.collectors.dell_ome.httpx.AsyncClient") as mock_cls:
            client = mock_cls.return_value.__aenter__.return_value
            client.post = AsyncMock(return_value=login_resp)
            client.get = AsyncMock(side_effect=[devices_page, contracts_page])

            results = await collector.collect(target)

        assert len(results) == 2
        a, b = results
        assert a.fields["serial_number"] == "AAA1"
        assert a.fields["warranty_end"] == "2027-01-01"
        assert b.fields["serial_number"] == "BBB2"
        assert b.fields["warranty_end"] is None
        assert b.fields["status"] == "maintenance"

    @pytest.mark.asyncio
    async def test_collect_fail_soft_on_old_ome_without_supportassist(self):
        """OME 3.4 returns 404 on /SupportAssistService/Contracts.
        Devices must still be returned; warranty fields just stay empty."""
        collector = DellOMECollector()
        target = CollectTarget(
            endpoint="https://old-ome.example.com",
            credentials={"username": "admin", "password": "secret"},
            options={"verify_tls": False},
        )

        login_resp = AsyncMock(status_code=200)
        login_resp.headers = {"X-Auth-Token": "tok"}
        login_resp.raise_for_status = lambda: None
        login_resp.json = lambda: {}

        devices_page = _mk_resp({
            "value": [
                {"DeviceServiceTag": "OLD1", "Model": "R430", "DeviceName": "old", "Status": 1000},
            ],
        })

        # 404 on Contracts → contract fetch should fail-soft, NOT bubble
        not_found = httpx.Response(
            status_code=404, request=httpx.Request("GET", "https://old-ome.example.com/x"),
        )
        not_found_err = httpx.HTTPStatusError(
            "Not Found", request=not_found.request, response=not_found,
        )
        contracts_resp = AsyncMock()
        contracts_resp.raise_for_status = AsyncMock(side_effect=not_found_err)
        contracts_resp.json = lambda: {}

        with patch("app.collectors.dell_ome.httpx.AsyncClient") as mock_cls:
            client = mock_cls.return_value.__aenter__.return_value
            client.post = AsyncMock(return_value=login_resp)
            client.get = AsyncMock(side_effect=[devices_page, contracts_resp])

            results = await collector.collect(target)

        # 1 device, no warranty data — that's the contract this test enforces
        assert len(results) == 1
        assert results[0].fields["serial_number"] == "OLD1"
        assert results[0].fields["warranty_end"] is None


def _mk_resp(payload: dict):
    resp = AsyncMock()
    resp.status_code = 200
    resp.json = lambda: payload
    resp.raise_for_status = lambda: None
    return resp
