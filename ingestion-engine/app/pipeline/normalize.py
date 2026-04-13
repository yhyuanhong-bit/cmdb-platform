"""Normalize raw asset data: lowercase keys, strip whitespace, resolve aliases."""

from app.models.common import RawAssetData

FIELD_ALIASES: dict[str, str] = {
    "serial": "serial_number",
    "hostname": "name",
    "host_name": "name",
    "manufacturer": "vendor",
    "mfg": "vendor",
    "device_type": "type",
    "asset_type": "type",
    "tag": "asset_tag",
    "bia": "bia_level",
    "business_impact": "bia_level",
    "model_number": "model",
    "prop_number": "property_number",
    "ctrl_number": "control_number",
    "subtype": "sub_type",
}

VALID_ASSET_FIELDS: set[str] = {
    "asset_tag",
    "name",
    "type",
    "sub_type",
    "status",
    "bia_level",
    "vendor",
    "model",
    "serial_number",
    "property_number",
    "control_number",
}


def normalize(raw: RawAssetData) -> RawAssetData:
    """Normalize field names and move unknown fields to attributes."""
    normalized_fields: dict[str, str | None] = {}
    extra_attributes: dict[str, str | None] = {}

    for key, value in raw.fields.items():
        # Lowercase and strip the key
        clean_key = key.lower().strip()

        # Strip whitespace from value if it's a string
        clean_value = value.strip() if isinstance(value, str) else value

        # Resolve aliases
        canonical = FIELD_ALIASES.get(clean_key, clean_key)

        if canonical in VALID_ASSET_FIELDS:
            normalized_fields[canonical] = clean_value
        else:
            extra_attributes[canonical] = clean_value

    # Merge extra attributes with existing attributes
    merged_attributes = dict(raw.attributes) if raw.attributes else {}
    merged_attributes.update(extra_attributes)

    return RawAssetData(
        source=raw.source,
        unique_key=raw.unique_key,
        fields=normalized_fields,
        attributes=merged_attributes if merged_attributes else None,
        collected_at=raw.collected_at,
    )
