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
    # English aliases for new fields
    "ip": "ip_address",
    "address": "ip_address",
    "ip_addr": "ip_address",
    "location": "location_name",
    "site": "location_name",
    "rack": "rack_name",
    "cabinet": "rack_name",
    "tags": "tags",
    "label": "tags",
    # Chinese aliases (Traditional)
    "資產編號": "asset_tag",
    "設備編號": "asset_tag",
    "名稱": "name",
    "設備名稱": "name",
    "類型": "type",
    "設備類型": "type",
    "子類型": "sub_type",
    "狀態": "status",
    "影響等級": "bia_level",
    "廠牌": "vendor",
    "廠商": "vendor",
    "製造商": "vendor",
    "型號": "model",
    "序號": "serial_number",
    "序列號": "serial_number",
    "財產編號": "property_number",
    "管制編號": "control_number",
    "IP地址": "ip_address",
    "IP": "ip_address",
    "位置": "location_name",
    "機房": "location_name",
    "機櫃": "rack_name",
    "標籤": "tags",
    # Simplified Chinese aliases
    "资产编号": "asset_tag",
    "设备编号": "asset_tag",
    "设备名称": "name",
    "设备类型": "type",
    "子类型": "sub_type",
    "状态": "status",
    "影响等级": "bia_level",
    "厂牌": "vendor",
    "厂商": "vendor",
    "制造商": "vendor",
    "型号": "model",
    "序列号": "serial_number",
    "财产编号": "property_number",
    "管制编号": "control_number",
    "位置": "location_name",
    "机房": "location_name",
    "机柜": "rack_name",
    "标签": "tags",
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
    "ip_address",
    "location_name",  # virtual: resolved to location_id in processor
    "rack_name",      # virtual: resolved to rack_id in processor
    "tags",
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
