"""Tests for discovery Celery task mode routing logic."""

import pytest

from app.models.common import RawAssetData
from app.tasks.discovery_task import determine_routing


def _make_raw(source: str = "snmp", unique_key: str = "SN1", serial: str = "SN1") -> RawAssetData:
    return RawAssetData(source=source, unique_key=unique_key, fields={"serial_number": serial})


# ──────────────────────────────────────────────
# determine_routing — pure-function tests
# ──────────────────────────────────────────────

def test_route_auto_returns_pipeline():
    raw = _make_raw()
    assert determine_routing("auto", raw, existing_asset_id=None) == "pipeline"


def test_route_auto_existing_still_returns_pipeline():
    """auto mode always routes to pipeline, even when asset already exists."""
    raw = _make_raw()
    assert determine_routing("auto", raw, existing_asset_id="some-uuid") == "pipeline"


def test_route_review_returns_staging():
    raw = _make_raw()
    assert determine_routing("review", raw, existing_asset_id=None) == "staging"


def test_route_review_existing_still_returns_staging():
    """review mode always routes to staging, even when asset already exists."""
    raw = _make_raw()
    assert determine_routing("review", raw, existing_asset_id="some-uuid") == "staging"


def test_route_smart_new_asset_returns_staging():
    """smart mode: no existing asset → staging for human review."""
    raw = _make_raw()
    assert determine_routing("smart", raw, existing_asset_id=None) == "staging"


def test_route_smart_existing_asset_returns_pipeline():
    """smart mode: known asset → pipeline for automated update."""
    raw = _make_raw()
    assert determine_routing("smart", raw, existing_asset_id="some-uuid") == "pipeline"


def test_route_smart_uuid_object_returns_pipeline():
    """smart mode: existing_asset_id can be a UUID object (truthy)."""
    import uuid

    raw = _make_raw()
    asset_id = uuid.uuid4()
    assert determine_routing("smart", raw, existing_asset_id=asset_id) == "pipeline"
