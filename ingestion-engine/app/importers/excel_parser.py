"""Excel and CSV file parsers for asset import."""

import csv

import openpyxl

from app.models.common import RawAssetData
from app.models.import_job import ParsedRow, ParseResult
from app.pipeline.normalize import VALID_ASSET_FIELDS

EXPECTED_HEADERS = [
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
]

REQUIRED_FIELDS = {"asset_tag", "name", "type"}


def _normalize_header(header: str) -> str:
    """Normalize a header string: lowercase, strip, replace spaces with underscores."""
    return header.lower().strip().replace(" ", "_")


def _validate_row(row_num: int, data: dict[str, str | None]) -> ParsedRow:
    """Validate a single parsed row and return a ParsedRow with any errors."""
    errors: list[str] = []

    for field in REQUIRED_FIELDS:
        value = data.get(field)
        if not value or (isinstance(value, str) and not value.strip()):
            errors.append(f"Missing required field: {field}")

    return ParsedRow(
        row_num=row_num,
        data=data,
        errors=errors if errors else None,
    )


def parse_excel(file_path: str) -> ParseResult:
    """Parse an Excel file (.xlsx) into a ParseResult.

    Expects the first row to be headers. Validates required fields
    (asset_tag, name, type) for each data row.
    """
    wb = openpyxl.load_workbook(file_path, read_only=True, data_only=True)
    ws = wb.active

    rows_iter = ws.iter_rows()

    # First row = headers
    header_row = next(rows_iter, None)
    if header_row is None:
        wb.close()
        return ParseResult(total_rows=0, valid_rows=[], error_rows=[], preview=[])

    headers = [
        _normalize_header(str(cell.value)) if cell.value is not None else f"column_{i}"
        for i, cell in enumerate(header_row)
    ]

    valid_rows: list[ParsedRow] = []
    error_rows: list[ParsedRow] = []
    all_rows: list[ParsedRow] = []

    for row_num, row in enumerate(rows_iter, start=2):
        # Skip completely empty rows
        values = [cell.value for cell in row]
        if all(v is None for v in values):
            continue

        data: dict[str, str | None] = {}
        for header, cell in zip(headers, row):
            value = cell.value
            data[header] = str(value).strip() if value is not None else None

        parsed_row = _validate_row(row_num, data)
        all_rows.append(parsed_row)

        if parsed_row.errors:
            error_rows.append(parsed_row)
        else:
            valid_rows.append(parsed_row)

    wb.close()

    preview = all_rows[:20]

    return ParseResult(
        total_rows=len(all_rows),
        valid_rows=valid_rows,
        error_rows=error_rows,
        preview=preview,
    )


def parse_csv(file_path: str) -> ParseResult:
    """Parse a CSV file into a ParseResult.

    Uses csv.DictReader with normalized headers. Validates required fields
    (asset_tag, name, type) for each data row.
    """
    valid_rows: list[ParsedRow] = []
    error_rows: list[ParsedRow] = []
    all_rows: list[ParsedRow] = []

    with open(file_path, newline="", encoding="utf-8-sig") as f:
        reader = csv.DictReader(f)

        if reader.fieldnames is None:
            return ParseResult(total_rows=0, valid_rows=[], error_rows=[], preview=[])

        # Normalize headers
        header_map = {
            original: _normalize_header(original)
            for original in reader.fieldnames
        }

        for row_num, raw_row in enumerate(reader, start=2):
            data: dict[str, str | None] = {}
            for original_key, normalized_key in header_map.items():
                value = raw_row.get(original_key)
                data[normalized_key] = value.strip() if value else None

            # Skip empty rows
            if all(v is None for v in data.values()):
                continue

            parsed_row = _validate_row(row_num, data)
            all_rows.append(parsed_row)

            if parsed_row.errors:
                error_rows.append(parsed_row)
            else:
                valid_rows.append(parsed_row)

    preview = all_rows[:20]

    return ParseResult(
        total_rows=len(all_rows),
        valid_rows=valid_rows,
        error_rows=error_rows,
        preview=preview,
    )


def rows_to_raw_assets(
    rows: list[ParsedRow], source: str = "excel"
) -> list[RawAssetData]:
    """Convert parsed rows to RawAssetData for the pipeline.

    Known fields (from VALID_ASSET_FIELDS) go to .fields,
    unknown fields go to .attributes. The unique_key is
    serial_number if present, otherwise asset_tag.
    """
    result: list[RawAssetData] = []

    for row in rows:
        fields: dict[str, str | None] = {}
        attributes: dict[str, str | None] = {}

        for key, value in row.data.items():
            if key in VALID_ASSET_FIELDS:
                fields[key] = value
            else:
                attributes[key] = value

        unique_key = fields.get("serial_number") or fields.get("asset_tag") or ""

        result.append(
            RawAssetData(
                source=source,
                unique_key=unique_key,
                fields=fields,
                attributes=attributes if attributes else None,
            )
        )

    return result
