"""Unit tests for the SNMP collector (pure-function coverage, no network I/O)."""

import pytest

from app.collectors.base import Collector
from app.collectors.snmp import (
    SNMPCollector,
    detect_vendor,
    expand_cidrs,
    parse_sysdescr,
)


# ──────────────────────────────────────────────
# 1. Protocol conformance
# ──────────────────────────────────────────────

def test_snmp_collector_implements_protocol():
    """SNMPCollector must satisfy the Collector runtime-checkable protocol."""
    collector = SNMPCollector()
    assert isinstance(collector, Collector)


# ──────────────────────────────────────────────
# 2. Supported fields
# ──────────────────────────────────────────────

def test_snmp_supported_fields():
    """supported_fields() must expose the expected field names."""
    collector = SNMPCollector()
    fields = collector.supported_fields()
    field_names = {f.field_name for f in fields}

    expected = {"hostname", "ip_address", "vendor", "model", "os_version", "serial_number"}
    assert expected == field_names, f"Missing or extra fields: {field_names ^ expected}"


# ──────────────────────────────────────────────
# 3. Vendor detection
# ──────────────────────────────────────────────

@pytest.mark.parametrize(
    "oid,expected_vendor",
    [
        # Each registered prefix
        ("1.3.6.1.4.1.9.1.1208", "Cisco"),
        ("1.3.6.1.4.1.11.2.3.9.1", "HP"),
        ("1.3.6.1.4.1.674.10895.3000.1", "Dell"),
        ("1.3.6.1.4.1.2011.5.25.100", "Huawei"),
        ("1.3.6.1.4.1.2636.3.1.3.0", "Juniper"),
        # Unknown prefix
        ("1.3.6.1.4.1.99999.1.2", "Unknown"),
        # Empty string
        ("", "Unknown"),
    ],
)
def test_snmp_vendor_detection(oid: str, expected_vendor: str):
    assert detect_vendor(oid) == expected_vendor


# ──────────────────────────────────────────────
# 4. sysDescr parsing
# ──────────────────────────────────────────────

class TestParseSysDescr:
    def test_cisco_ios(self):
        descr = (
            "Cisco IOS Software, Version 15.2(4)M3, RELEASE SOFTWARE (fc2) "
            "Technical Support: http://www.cisco.com/techsupport"
        )
        result = parse_sysdescr(descr)
        assert result["os_version"] == "15.2(4)M3"
        assert result["model"] is not None  # model heuristic should fire

    def test_version_extracted(self):
        descr = "Linux switch Version 5.10.0-18-amd64 SMP Debian"
        result = parse_sysdescr(descr)
        assert result["os_version"] == "5.10.0-18-amd64"

    def test_empty_string(self):
        result = parse_sysdescr("")
        assert result == {"model": None, "os_version": None}

    def test_no_version_no_model(self):
        result = parse_sysdescr("Some generic description with no version info.")
        # Should not raise; keys must exist
        assert "model" in result
        assert "os_version" in result


def test_snmp_parse_sysdescr():
    """Alias test to satisfy the named-test requirement."""
    # Re-run canonical Cisco case as a standalone test
    descr = "Cisco IOS Software, Version 12.4(24)T4, RELEASE SOFTWARE"
    result = parse_sysdescr(descr)
    assert result["os_version"] == "12.4(24)T4"


# ──────────────────────────────────────────────
# 5. CIDR expansion
# ──────────────────────────────────────────────

def test_snmp_expand_cidrs():
    """/30 contains exactly 2 usable host addresses (network + broadcast excluded)."""
    ips = expand_cidrs(["192.168.1.0/30"])
    assert len(ips) == 2
    assert "192.168.1.1" in ips
    assert "192.168.1.2" in ips
    # Network and broadcast must not appear
    assert "192.168.1.0" not in ips
    assert "192.168.1.3" not in ips


def test_expand_cidrs_single_host():
    """/32 returns the single address."""
    ips = expand_cidrs(["10.0.0.1/32"])
    assert ips == ["10.0.0.1"]


def test_expand_cidrs_multiple():
    """Multiple CIDRs are concatenated correctly."""
    ips = expand_cidrs(["10.0.0.0/30", "10.0.1.0/30"])
    assert len(ips) == 4


def test_expand_cidrs_invalid_skipped():
    """Invalid entries are skipped without raising."""
    ips = expand_cidrs(["not-a-cidr", "10.0.0.0/30"])
    assert len(ips) == 2
