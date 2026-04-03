"""Tests for the authority check module."""

from app.pipeline.authority import AuthorityResult, _get_max_priority


def test_get_max_priority():
    """Verify _get_max_priority returns the highest priority for a field."""
    authorities = {
        ("serial_number", "ipmi"): 100,
        ("serial_number", "snmp"): 80,
        ("serial_number", "manual"): 50,
        ("vendor", "ipmi"): 100,
        ("vendor", "manual"): 50,
    }
    assert _get_max_priority(authorities, "serial_number") == 100
    assert _get_max_priority(authorities, "vendor") == 100
    assert _get_max_priority(authorities, "unknown_field") == 0


def test_authority_result_structure():
    """Verify AuthorityResult fields are initialized correctly."""
    result = AuthorityResult(
        auto_merge_fields={"name": "web-01"},
        conflict_fields=[
            {
                "field_name": "vendor",
                "current_value": "Dell",
                "incoming_value": "HP",
            }
        ],
        skipped_fields=["status"],
    )
    assert result.auto_merge_fields == {"name": "web-01"}
    assert len(result.conflict_fields) == 1
    assert result.conflict_fields[0]["field_name"] == "vendor"
    assert result.skipped_fields == ["status"]

    # Test defaults
    empty = AuthorityResult()
    assert empty.auto_merge_fields == {}
    assert empty.conflict_fields == []
    assert empty.skipped_fields == []
