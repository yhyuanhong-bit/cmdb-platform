"""Tests for structured error reporting in process_batch.

process_batch must record per-row errors with a rich structure:
    {
        "row": <int>,          # 1-based row index within the batch
        "reason": <str>,       # human-readable error message (str(exc))
        "field": <str|None>,   # offending field if inferable, else None
        "exception_type": <str>,  # exception class name
    }

The legacy "errors" counter (an integer) MUST be preserved via "error_count"
so downstream consumers that surface counts keep working.
"""

from __future__ import annotations

from uuid import UUID, uuid4

import pytest

from app.models.common import PipelineResult, RawAssetData
from app.pipeline import processor


def _raw(i: int) -> RawAssetData:
    return RawAssetData(
        source="excel",
        unique_key=f"SN-{i}",
        fields={"serial_number": f"SN-{i}", "name": f"host-{i}"},
    )


@pytest.fixture
def tenant_id() -> UUID:
    return uuid4()


# ──────────────────────────────────────────────
# Errors list — rich, structured per-row records
# ──────────────────────────────────────────────


async def test_process_batch_captures_structured_error(monkeypatch, tenant_id):
    """A failing row must produce a dict with row/reason/field/exception_type."""

    async def boom(pool, tenant, raw):
        # Simulate a validation-style failure carrying a field hint.
        err = ValueError("ip_address is malformed")
        err.field = "ip_address"  # optional attribute picked up by reporter
        raise err

    monkeypatch.setattr(processor, "process_single", boom)

    stats = await processor.process_batch(
        pool=None, tenant_id=tenant_id, items=[_raw(1)]
    )

    assert stats["error_count"] == 1
    assert isinstance(stats["errors"], list)
    assert len(stats["errors"]) == 1

    err_record = stats["errors"][0]
    assert err_record["row"] == 1
    assert err_record["reason"] == "ip_address is malformed"
    assert err_record["field"] == "ip_address"
    assert err_record["exception_type"] == "ValueError"


async def test_process_batch_field_defaults_to_none_when_unknown(
    monkeypatch, tenant_id
):
    """When no field hint is available, field must be None (not absent)."""

    async def boom(pool, tenant, raw):
        raise RuntimeError("db connection dropped")

    monkeypatch.setattr(processor, "process_single", boom)

    stats = await processor.process_batch(
        pool=None, tenant_id=tenant_id, items=[_raw(1)]
    )

    err_record = stats["errors"][0]
    assert err_record["row"] == 1
    assert err_record["reason"] == "db connection dropped"
    assert err_record["field"] is None
    assert err_record["exception_type"] == "RuntimeError"


async def test_process_batch_records_multiple_errors_with_row_numbers(
    monkeypatch, tenant_id
):
    """Row numbers must be 1-based and errors must stay ordered."""

    async def sometimes_boom(pool, tenant, raw):
        if raw.unique_key == "SN-2":
            raise ValueError("bad row 2")
        if raw.unique_key == "SN-4":
            err = KeyError("missing")
            err.field = "serial_number"
            raise err
        return PipelineResult(asset_id=uuid4(), action="created")

    monkeypatch.setattr(processor, "process_single", sometimes_boom)

    items = [_raw(1), _raw(2), _raw(3), _raw(4)]
    stats = await processor.process_batch(
        pool=None, tenant_id=tenant_id, items=items
    )

    assert stats["created"] == 2
    assert stats["error_count"] == 2
    assert len(stats["errors"]) == 2

    first, second = stats["errors"]
    assert first["row"] == 2
    assert first["exception_type"] == "ValueError"
    assert first["reason"] == "bad row 2"
    assert first["field"] is None

    assert second["row"] == 4
    assert second["exception_type"] == "KeyError"
    assert second["field"] == "serial_number"


async def test_process_batch_errors_empty_when_all_succeed(
    monkeypatch, tenant_id
):
    async def ok(pool, tenant, raw):
        return PipelineResult(asset_id=uuid4(), action="created")

    monkeypatch.setattr(processor, "process_single", ok)

    stats = await processor.process_batch(
        pool=None, tenant_id=tenant_id, items=[_raw(1), _raw(2)]
    )

    assert stats["created"] == 2
    assert stats["error_count"] == 0
    assert stats["errors"] == []
