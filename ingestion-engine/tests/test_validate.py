"""Tests for the validate pipeline stage."""

from app.models.common import RawAssetData
from app.pipeline.validate import validate_for_create, validate_for_update


def test_missing_required_fields_for_create():
    """Create validation rejects data missing required fields."""
    raw = RawAssetData(
        source="test",
        unique_key="k1",
        fields={"vendor": "Dell"},
    )
    errors = validate_for_create(raw)
    assert len(errors) == 3
    assert any("asset_tag" in e for e in errors)
    assert any("name" in e for e in errors)
    assert any("type" in e for e in errors)


def test_valid_create_passes():
    """Create validation passes with all required fields and valid values."""
    raw = RawAssetData(
        source="test",
        unique_key="k2",
        fields={
            "asset_tag": "A-001",
            "name": "web-01",
            "type": "server",
            "status": "deployed",
            "bia_level": "critical",
        },
    )
    errors = validate_for_create(raw)
    assert errors == []


def test_invalid_type_rejected():
    """Create validation rejects an invalid asset type."""
    raw = RawAssetData(
        source="test",
        unique_key="k3",
        fields={
            "asset_tag": "A-002",
            "name": "mystery-box",
            "type": "toaster",
        },
    )
    errors = validate_for_create(raw)
    assert len(errors) == 1
    assert "toaster" in errors[0]


def test_update_allows_partial_fields():
    """Update validation allows partial data (no required fields check)."""
    raw = RawAssetData(
        source="test",
        unique_key="k4",
        fields={"vendor": "HP", "model": "ProLiant"},
    )
    errors = validate_for_update(raw)
    assert errors == []


def test_update_rejects_invalid_status():
    """Update validation still rejects invalid enum values."""
    raw = RawAssetData(
        source="test",
        unique_key="k5",
        fields={"status": "exploded"},
    )
    errors = validate_for_update(raw)
    assert len(errors) == 1
    assert "exploded" in errors[0]
