"""Validate raw asset data before create or update."""

from app.models.common import RawAssetData

REQUIRED_FIELDS_FOR_CREATE: set[str] = {"asset_tag", "name", "type"}

VALID_TYPES: set[str] = {"server", "network", "storage", "power"}
VALID_STATUSES: set[str] = {
    "inventoried",
    "deployed",
    "operational",
    "maintenance",
    "decommissioned",
}
VALID_BIA_LEVELS: set[str] = {"critical", "important", "normal", "minor"}


def validate_for_create(raw: RawAssetData) -> list[str]:
    """Validate data for creating a new asset. Returns list of error messages."""
    errors: list[str] = []

    # Check required fields
    for field in REQUIRED_FIELDS_FOR_CREATE:
        value = raw.fields.get(field)
        if not value:
            errors.append(f"Missing required field: {field}")

    # Validate type if provided
    asset_type = raw.fields.get("type")
    if asset_type and asset_type not in VALID_TYPES:
        errors.append(
            f"Invalid type '{asset_type}'. Must be one of: {', '.join(sorted(VALID_TYPES))}"
        )

    # Validate status if provided
    status = raw.fields.get("status")
    if status and status not in VALID_STATUSES:
        errors.append(
            f"Invalid status '{status}'. Must be one of: {', '.join(sorted(VALID_STATUSES))}"
        )

    # Validate bia_level if provided
    bia_level = raw.fields.get("bia_level")
    if bia_level and bia_level not in VALID_BIA_LEVELS:
        errors.append(
            f"Invalid bia_level '{bia_level}'. Must be one of: {', '.join(sorted(VALID_BIA_LEVELS))}"
        )

    return errors


def validate_for_update(raw: RawAssetData) -> list[str]:
    """Validate data for updating an existing asset. Less strict - only checks format of provided fields."""
    errors: list[str] = []

    # Validate type if provided
    asset_type = raw.fields.get("type")
    if asset_type and asset_type not in VALID_TYPES:
        errors.append(
            f"Invalid type '{asset_type}'. Must be one of: {', '.join(sorted(VALID_TYPES))}"
        )

    # Validate status if provided
    status = raw.fields.get("status")
    if status and status not in VALID_STATUSES:
        errors.append(
            f"Invalid status '{status}'. Must be one of: {', '.join(sorted(VALID_STATUSES))}"
        )

    # Validate bia_level if provided
    bia_level = raw.fields.get("bia_level")
    if bia_level and bia_level not in VALID_BIA_LEVELS:
        errors.append(
            f"Invalid bia_level '{bia_level}'. Must be one of: {', '.join(sorted(VALID_BIA_LEVELS))}"
        )

    return errors
