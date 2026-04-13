"""Unit tests for the IPMI collector."""

from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from app.collectors.base import Collector
from app.collectors.ipmi import IPMICollector, parse_fru_inventory
from app.models.common import CollectTarget, FieldMapping, RawAssetData


# ---------------------------------------------------------------------------
# Protocol conformance
# ---------------------------------------------------------------------------

class TestIPMICollectorProtocol:
    def test_ipmi_collector_implements_protocol(self):
        """IPMICollector must satisfy the Collector protocol."""
        collector = IPMICollector()
        assert isinstance(collector, Collector)

    def test_collector_name_and_type(self):
        """Collector must expose the correct name and collect_type."""
        collector = IPMICollector()
        assert collector.name == "ipmi"
        assert collector.collect_type == "ipmi"


# ---------------------------------------------------------------------------
# Supported fields
# ---------------------------------------------------------------------------

class TestIPMISupportedFields:
    def test_ipmi_supported_fields(self):
        """supported_fields must return FieldMapping objects with expected names."""
        collector = IPMICollector()
        fields = collector.supported_fields()

        assert isinstance(fields, list)
        assert len(fields) > 0
        assert all(isinstance(f, FieldMapping) for f in fields)

        names = {f.field_name for f in fields}
        expected = {
            "serial_number",
            "vendor",
            "model",
            "power_state",
            "bmc_ip",
            "bmc_mac",
            "firmware_version",
        }
        assert expected.issubset(names), f"Missing fields: {expected - names}"

    def test_serial_number_is_authoritative(self):
        """serial_number should be flagged as an authoritative field."""
        collector = IPMICollector()
        fields = {f.field_name: f for f in collector.supported_fields()}
        assert fields["serial_number"].authority is True

    def test_vendor_is_authoritative(self):
        """vendor should be flagged as an authoritative field."""
        collector = IPMICollector()
        fields = {f.field_name: f for f in collector.supported_fields()}
        assert fields["vendor"].authority is True

    def test_model_is_authoritative(self):
        """model should be flagged as an authoritative field."""
        collector = IPMICollector()
        fields = {f.field_name: f for f in collector.supported_fields()}
        assert fields["model"].authority is True


# ---------------------------------------------------------------------------
# parse_fru_inventory — full data
# ---------------------------------------------------------------------------

class TestParseFruInventory:
    def test_parse_fru_inventory(self):
        """Full FRU dict should map to all expected fields and attributes."""
        fru = {
            "product_serial": "SN-1234567",
            "product_manufacturer": "SuperMicro",
            "product_name": "SYS-6019P-WTR",
            "product_part_number": "P/N-ABC-999",
        }
        fields, attributes = parse_fru_inventory(fru)

        assert fields.get("serial_number") == "SN-1234567"
        assert fields.get("vendor") == "SuperMicro"
        assert fields.get("model") == "SYS-6019P-WTR"
        assert attributes.get("part_number") == "P/N-ABC-999"

    def test_parse_fru_inventory_alternative_keys(self):
        """Alternative pyghmi key names should also be handled."""
        fru = {
            "serial_number": "ALT-SN-999",
            "manufacturer": "Dell",
            "Product Name": "PowerEdge R740",
            "Part Number": "0H755M",
        }
        fields, attributes = parse_fru_inventory(fru)

        assert fields.get("serial_number") == "ALT-SN-999"
        assert fields.get("vendor") == "Dell"
        assert fields.get("model") == "PowerEdge R740"
        assert attributes.get("part_number") == "0H755M"

    def test_parse_fru_inventory_display_name_keys(self):
        """Human-readable FRU key names (e.g. 'Serial Number') should be parsed."""
        fru = {
            "Serial Number": "DISP-SN-001",
            "Manufacturer": "HPE",
        }
        fields, attributes = parse_fru_inventory(fru)

        assert fields.get("serial_number") == "DISP-SN-001"
        assert fields.get("vendor") == "HPE"

    def test_parse_fru_inventory_returns_tuple(self):
        """Return value must always be a (dict, dict) tuple."""
        result = parse_fru_inventory({})
        assert isinstance(result, tuple)
        assert len(result) == 2
        fields, attributes = result
        assert isinstance(fields, dict)
        assert isinstance(attributes, dict)


# ---------------------------------------------------------------------------
# parse_fru_inventory — missing / partial data
# ---------------------------------------------------------------------------

class TestParseFruInventoryMissingFields:
    def test_parse_fru_inventory_missing_fields(self):
        """Partial FRU dict should not raise; missing keys absent from output."""
        fru = {
            "product_name": "Generic Server",
        }
        fields, attributes = parse_fru_inventory(fru)

        assert fields.get("model") == "Generic Server"
        assert "serial_number" not in fields
        assert "vendor" not in fields
        assert "part_number" not in attributes

    def test_parse_fru_inventory_empty_dict(self):
        """Empty FRU dict should return empty fields and attributes."""
        fields, attributes = parse_fru_inventory({})

        assert fields == {}
        assert attributes == {}

    def test_parse_fru_inventory_none_values_ignored(self):
        """Keys present but set to None should not populate output fields."""
        fru = {
            "product_serial": None,
            "product_manufacturer": None,
            "product_name": "MyServer",
        }
        fields, attributes = parse_fru_inventory(fru)

        assert "serial_number" not in fields
        assert "vendor" not in fields
        assert fields.get("model") == "MyServer"

    def test_parse_fru_inventory_only_part_number(self):
        """Part number alone should appear in attributes, not fields."""
        fru = {"product_part_number": "XYZ-001"}
        fields, attributes = parse_fru_inventory(fru)

        assert fields == {}
        assert attributes.get("part_number") == "XYZ-001"


# ---------------------------------------------------------------------------
# Async collect — integration-level with mocked pyghmi
# ---------------------------------------------------------------------------

class TestIPMICollectorCollect:
    @pytest.mark.asyncio
    async def test_collect_returns_list(self):
        """collect() must always return a list (even on connection failure)."""
        collector = IPMICollector()
        target = CollectTarget(
            endpoint="192.0.2.1",
            credentials={"username": "admin", "password": "pass"},
        )

        with patch("app.collectors.ipmi._collect_single_sync", return_value=None):
            results = await collector.collect(target)

        assert isinstance(results, list)

    @pytest.mark.asyncio
    async def test_collect_returns_raw_asset_data(self):
        """collect() should include RawAssetData returned by _collect_single_sync."""
        collector = IPMICollector()
        target = CollectTarget(
            endpoint="192.0.2.1",
            credentials={"username": "admin", "password": "pass"},
        )
        fake_asset = RawAssetData(
            source="ipmi",
            unique_key="SN-TEST-001",
            fields={"serial_number": "SN-TEST-001", "vendor": "Dell"},
        )

        with patch("app.collectors.ipmi._collect_single_sync", return_value=fake_asset):
            results = await collector.collect(target)

        assert len(results) == 1
        assert results[0].unique_key == "SN-TEST-001"
        assert results[0].fields["vendor"] == "Dell"

    @pytest.mark.asyncio
    async def test_collect_multiple_ips(self):
        """collect() should iterate all IPs in a /30 CIDR."""
        collector = IPMICollector()
        target = CollectTarget(
            endpoint="10.0.0.0/30",  # hosts: 10.0.0.1, 10.0.0.2
            credentials={"username": "admin", "password": "pass"},
        )

        call_count = 0

        def fake_collect(ip, creds, opts):
            nonlocal call_count
            call_count += 1
            return RawAssetData(
                source="ipmi",
                unique_key=ip,
                fields={"bmc_ip": ip},
            )

        with patch("app.collectors.ipmi._collect_single_sync", side_effect=fake_collect):
            results = await collector.collect(target)

        assert call_count == 2
        assert len(results) == 2
