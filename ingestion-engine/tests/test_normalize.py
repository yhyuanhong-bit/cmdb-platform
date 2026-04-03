"""Tests for the normalize pipeline stage."""

from app.models.common import RawAssetData
from app.pipeline.normalize import normalize


def test_aliases_resolve_to_canonical_names():
    """Field aliases like 'serial', 'hostname', 'manufacturer' resolve correctly."""
    raw = RawAssetData(
        source="test",
        unique_key="k1",
        fields={
            "serial": "ABC123",
            "hostname": "web-01",
            "manufacturer": "Dell",
            "device_type": "server",
            "tag": "A-001",
            "bia": "critical",
        },
    )
    result = normalize(raw)
    assert result.fields["serial_number"] == "ABC123"
    assert result.fields["name"] == "web-01"
    assert result.fields["vendor"] == "Dell"
    assert result.fields["type"] == "server"
    assert result.fields["asset_tag"] == "A-001"
    assert result.fields["bia_level"] == "critical"


def test_whitespace_stripped_from_keys_and_values():
    """Leading/trailing whitespace is removed from both keys and values."""
    raw = RawAssetData(
        source="test",
        unique_key="k2",
        fields={
            "  Name  ": "  web-02  ",
            " Asset_Tag": " B-002 ",
        },
    )
    result = normalize(raw)
    assert result.fields["name"] == "web-02"
    assert result.fields["asset_tag"] == "B-002"


def test_unknown_fields_go_to_attributes():
    """Fields not in VALID_ASSET_FIELDS are moved to attributes."""
    raw = RawAssetData(
        source="test",
        unique_key="k3",
        fields={
            "name": "web-03",
            "rack_location": "R12-U5",
            "custom_note": "needs review",
        },
    )
    result = normalize(raw)
    assert result.fields == {"name": "web-03"}
    assert result.attributes["rack_location"] == "R12-U5"
    assert result.attributes["custom_note"] == "needs review"
