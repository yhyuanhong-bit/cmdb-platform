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
    # BMC/IPMI English aliases
    "bmc": "bmc_ip",
    "bmc_address": "bmc_ip",
    "ilo": "bmc_ip",
    "ilo_ip": "bmc_ip",
    "idrac": "bmc_ip",
    "idrac_ip": "bmc_ip",
    "ipmi": "bmc_ip",
    "ipmi_ip": "bmc_ip",
    "bmc_version": "bmc_firmware",
    "firmware": "bmc_firmware",
    "ilo_version": "bmc_firmware",
    "idrac_version": "bmc_firmware",
    "management_type": "bmc_type",
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
    # BMC Chinese aliases (Traditional)
    "管理IP": "bmc_ip",
    "帶外管理IP": "bmc_ip",
    "iLO IP": "bmc_ip",
    "iDRAC IP": "bmc_ip",
    "管理介面": "bmc_type",
    "帶外管理類型": "bmc_type",
    "韌體版本": "bmc_firmware",
    "BMC韌體": "bmc_firmware",
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
    # BMC Chinese aliases (Simplified)
    "带外管理IP": "bmc_ip",
    "管理接口": "bmc_type",
    "带外管理类型": "bmc_type",
    "固件版本": "bmc_firmware",
    "BMC固件": "bmc_firmware",
    # Lifecycle English
    "purchase": "purchase_date",
    "bought_date": "purchase_date",
    "cost": "purchase_cost",
    "price": "purchase_cost",
    "warranty_from": "warranty_start",
    "warranty_to": "warranty_end",
    "warranty_expiry": "warranty_end",
    "warranty_expires": "warranty_end",
    "warranty_provider": "warranty_vendor",
    "contract": "warranty_contract",
    "contract_number": "warranty_contract",
    "contract_no": "warranty_contract",
    "lifespan": "expected_lifespan_months",
    "useful_life": "expected_lifespan_months",
    "end_of_life": "eol_date",
    # Traditional Chinese
    "採購日期": "purchase_date",
    "購買日期": "purchase_date",
    "採購金額": "purchase_cost",
    "購買金額": "purchase_cost",
    "保固開始": "warranty_start",
    "保固起始": "warranty_start",
    "保固到期": "warranty_end",
    "保固結束": "warranty_end",
    "保固廠商": "warranty_vendor",
    "保固供應商": "warranty_vendor",
    "合約編號": "warranty_contract",
    "保固合約": "warranty_contract",
    "預計使用月數": "expected_lifespan_months",
    "使用年限": "expected_lifespan_months",
    "報廢日期": "eol_date",
    "EOL日期": "eol_date",
    # Simplified Chinese
    "采购日期": "purchase_date",
    "购买日期": "purchase_date",
    "采购金额": "purchase_cost",
    "购买金额": "purchase_cost",
    "保固开始": "warranty_start",
    "质保开始": "warranty_start",
    "质保到期": "warranty_end",
    "保固厂商": "warranty_vendor",
    "质保供应商": "warranty_vendor",
    "合同编号": "warranty_contract",
    "质保合同": "warranty_contract",
    "预计使用月数": "expected_lifespan_months",
    "报废日期": "eol_date",
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
    "bmc_ip",
    "bmc_type",
    "bmc_firmware",
    "purchase_date",
    "purchase_cost",
    "warranty_start",
    "warranty_end",
    "warranty_vendor",
    "warranty_contract",
    "expected_lifespan_months",
    "eol_date",
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
