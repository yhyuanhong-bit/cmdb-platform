import { memo } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useInventoryTask, useInventoryItems } from "../hooks/useInventory";

/* ──────────────────────────────────────────────
   Static fallback data (used when API returns no detail)
   ────────────────────────────────────────────── */

const FALLBACK_ASSET = {
  name: "NODE-4483-B3",
  tag: "IDC01-B3-S04-P1",
  serialNumber: "SN-2023-XK7R-4483",
  model: "Dell PowerEdge R750xs",
  manufacturer: "Dell Technologies",
  location: "IDC-01 / Room B / Rack B3 / Slot 04 / Position 1",
  status: "Powered Off",
  expectedStatus: "Active",
  owner: "Infrastructure Team",
  purchaseDate: "2023-06-15",
  warrantyExpiry: "2026-06-15",
  lastMaintenance: "2025-12-10",
  ipAddress: "10.128.3.44",
  macAddress: "A4:BF:01:23:45:67",
};

const SCAN_HISTORY = [
  {
    timestamp: "2026-03-28 14:18:32",
    operator: "Chen, Wei-Lin",
    method: "QR Scan",
    result: "status_mismatch",
    note: "Device powered off, expected active state",
  },
  {
    timestamp: "2025-12-10 09:45:00",
    operator: "Lin, Mei-Hua",
    method: "QR Scan",
    result: "match",
    note: "Routine quarterly audit - all fields match",
  },
  {
    timestamp: "2025-09-15 11:22:17",
    operator: "Wang, Jun",
    method: "Manual Entry",
    result: "match",
    note: "Q3 2025 inventory check passed",
  },
  {
    timestamp: "2025-06-20 16:05:44",
    operator: "Chen, Wei-Lin",
    method: "QR Scan",
    result: "location_update",
    note: "Moved from Rack A2/Slot 12 to Rack B3/Slot 04",
  },
  {
    timestamp: "2025-03-08 10:30:00",
    operator: "Huang, Xiao-Ming",
    method: "QR Scan",
    result: "match",
    note: "Initial deployment verification",
  },
];

const DISCREPANCY_NOTES = [
  {
    id: "DISC-2026-0328-001",
    timestamp: "2026-03-28 14:20:05",
    author: "Chen, Wei-Lin",
    severity: "HIGH",
    text: "Asset scanned as powered off but CMDB record shows Active status. Physical inspection confirms power LED is off. No network connectivity on assigned IP 10.128.3.44. Requires investigation by infrastructure team.",
  },
  {
    id: "DISC-2026-0328-002",
    timestamp: "2026-03-28 14:22:30",
    author: "System (Auto)",
    severity: "INFO",
    text: "Cross-reference check: Last known active ping was 2026-03-26 03:14 UTC. Monitoring system shows host unreachable since 2026-03-26 03:15 UTC. Potential unplanned shutdown.",
  },
];

/* ──────────────────────────────────────────────
   Small reusable pieces
   ────────────────────────────────────────────── */

function Icon({ name, className = "" }: { name: string; className?: string }) {
  return (
    <span className={`material-symbols-outlined ${className}`}>{name}</span>
  );
}

function InfoRow({ label, value, highlight }: { label: string; value: string; highlight?: boolean }) {
  return (
    <div className="flex items-start justify-between py-2.5">
      <span className="text-xs text-on-surface-variant font-label uppercase tracking-widest shrink-0 w-40">
        {label}
      </span>
      <span
        className={`text-sm text-right font-body ${
          highlight ? "text-error font-bold" : "text-on-surface"
        }`}
      >
        {value}
      </span>
    </div>
  );
}

/* ──────────────────────────────────────────────
   Main page
   ────────────────────────────────────────────── */

const InventoryItemDetail = memo(function InventoryItemDetail() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const taskId = searchParams.get('taskId') ?? '';

  const { data: taskResponse, isLoading: taskLoading } = useInventoryTask(taskId);
  const { data: itemsResponse } = useInventoryItems(taskId);
  const task = taskResponse?.data;
  const items = itemsResponse?.data ?? [];
  const firstItem = items[0];

  // Build ASSET from API data or fall back to static
  const ASSET = task ? {
    name: task.name || FALLBACK_ASSET.name,
    tag: task.code || FALLBACK_ASSET.tag,
    serialNumber: firstItem?.asset_id ?? FALLBACK_ASSET.serialNumber,
    model: FALLBACK_ASSET.model,
    manufacturer: FALLBACK_ASSET.manufacturer,
    location: FALLBACK_ASSET.location,
    status: firstItem?.status ?? FALLBACK_ASSET.status,
    expectedStatus: FALLBACK_ASSET.expectedStatus,
    owner: FALLBACK_ASSET.owner,
    purchaseDate: FALLBACK_ASSET.purchaseDate,
    warrantyExpiry: FALLBACK_ASSET.warrantyExpiry,
    lastMaintenance: FALLBACK_ASSET.lastMaintenance,
    ipAddress: FALLBACK_ASSET.ipAddress,
    macAddress: FALLBACK_ASSET.macAddress,
  } : FALLBACK_ASSET;

  if (taskLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-primary border-t-transparent" />
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-surface p-6 font-body text-on-surface">
      {/* Header */}
      <div className="flex items-start justify-between mb-6">
        <div>
          <button
            onClick={() => navigate('/inventory')}
            className="text-xs text-on-surface-variant font-label flex items-center gap-1 mb-3 hover:text-primary transition-colors"
          >
            <Icon name="arrow_back" className="text-sm" />
            {t('inventory_detail.back_to_inventory_task')}
          </button>
          <div className="flex items-center gap-3">
            <h1
              className="font-headline text-3xl font-bold tracking-tight text-on-surface cursor-pointer hover:text-primary transition-colors"
              onClick={() => navigate('/assets/detail')}
              title="View asset detail"
            >
              {ASSET.name}
            </h1>
            <span className="bg-error-container text-error text-[10px] font-label font-bold tracking-widest px-3 py-1 rounded-lg">
              {t('inventory_detail.mismatch')}
            </span>
          </div>
          <div className="flex items-center gap-4 mt-2">
            <span className="text-xs text-on-surface-variant font-label">
              {t('inventory_detail.tag')}:{" "}
              <span className="text-on-surface tabular-nums">{ASSET.tag}</span>
            </span>
            <span className="text-xs text-on-surface-variant font-label flex items-center gap-1">
              <Icon name="qr_code" className="text-sm text-primary" />
              {t('inventory_detail.qr_verified')}
            </span>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <button className="bg-primary hover:opacity-90 text-on-primary px-4 py-2 rounded-xl text-sm font-label font-bold flex items-center gap-2 transition-opacity">
            <Icon name="verified" className="text-lg" />
            {t('inventory_detail.verify_asset')}
          </button>
          <button className="bg-tertiary-container hover:opacity-90 text-tertiary px-4 py-2 rounded-xl text-sm font-label font-bold flex items-center gap-2 transition-opacity">
            <Icon name="flag" className="text-lg" />
            {t('inventory_detail.flag_issue')}
          </button>
          <button className="bg-surface-container-high hover:bg-surface-container-highest text-on-surface-variant px-4 py-2 rounded-xl text-sm font-label font-bold flex items-center gap-2 transition-colors">
            <Icon name="print" className="text-lg" />
            {t('inventory_detail.print_label')}
          </button>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-4 mb-6">
        {/* Asset Information */}
        <div className="col-span-2 bg-surface-container rounded-xl p-5">
          <div className="flex items-center gap-2 mb-4">
            <Icon name="info" className="text-primary text-xl" />
            <h2 className="font-headline text-base font-bold text-on-surface">
              {t('inventory_detail.asset_information')}
            </h2>
          </div>

          <div className="grid grid-cols-2 gap-x-8">
            <div>
              <div className="flex items-start justify-between py-2.5">
                <span className="text-xs text-on-surface-variant font-label uppercase tracking-widest shrink-0 w-40">
                  {t('inventory_detail.serial_number')}
                </span>
                <span
                  className="text-sm text-right font-body text-primary cursor-pointer hover:underline"
                  onClick={() => navigate('/assets/detail')}
                >
                  {ASSET.serialNumber}
                </span>
              </div>
              <InfoRow label={t('inventory_detail.model')} value={ASSET.model} />
              <InfoRow label={t('inventory_detail.manufacturer')} value={ASSET.manufacturer} />
              <InfoRow label={t('common.location')} value={ASSET.location} />
              <InfoRow
                label={t('inventory_detail.current_status')}
                value={ASSET.status}
                highlight
              />
              <InfoRow
                label={t('inventory_detail.expected_status')}
                value={ASSET.expectedStatus}
              />
            </div>
            <div>
              <InfoRow label="Owner" value={ASSET.owner} />
              <InfoRow label={t('inventory_detail.purchase_date')} value={ASSET.purchaseDate} />
              <InfoRow label={t('inventory_detail.warranty_expiry')} value={ASSET.warrantyExpiry} />
              <InfoRow label={t('inventory_detail.last_maintenance')} value={ASSET.lastMaintenance} />
              <InfoRow label={t('inventory_detail.ip_address')} value={ASSET.ipAddress} />
              <InfoRow label={t('inventory_detail.mac_address')} value={ASSET.macAddress} />
            </div>
          </div>

          {/* Status mismatch alert */}
          <div className="mt-4 bg-error-container/30 rounded-xl p-4 flex items-start gap-3">
            <Icon name="warning" className="text-error text-xl shrink-0 mt-0.5" />
            <div>
              <p className="text-sm font-headline font-bold text-error mb-1">
                {t('inventory_detail.status_discrepancy_detected')}
              </p>
              <p className="text-xs text-on-surface-variant leading-relaxed">
                CMDB record indicates this asset should be{" "}
                <span className="text-[#69db7c] font-bold">Active</span>, but
                physical scan confirms the device is{" "}
                <span className="text-error font-bold">Powered Off</span>.
                Last network activity detected on 2026-03-26 03:14 UTC.
              </p>
            </div>
          </div>
        </div>

        {/* QR / Barcode panel */}
        <div className="bg-surface-container rounded-xl p-5 flex flex-col">
          <div className="flex items-center gap-2 mb-4">
            <Icon name="qr_code_2" className="text-primary text-xl" />
            <h2 className="font-headline text-base font-bold text-on-surface">
              {t('inventory_detail.asset_identifier')}
            </h2>
          </div>

          {/* QR placeholder */}
          <div className="bg-surface-container-low rounded-xl p-6 flex flex-col items-center gap-3 mb-4">
            <div className="w-32 h-32 bg-surface-container-high rounded-xl flex items-center justify-center">
              <Icon name="qr_code_2" className="text-5xl text-on-surface-variant/40" />
            </div>
            <span className="text-[10px] text-on-surface-variant font-label tracking-widest uppercase">
              {t('inventory_detail.qr_code')}
            </span>
            <span className="text-xs text-on-surface font-label tabular-nums">
              {ASSET.tag}
            </span>
          </div>

          {/* Barcode placeholder */}
          <div className="bg-surface-container-low rounded-xl p-4 flex flex-col items-center gap-2">
            <div className="w-full h-12 bg-surface-container-high rounded-lg flex items-center justify-center gap-[2px] px-4">
              {Array.from({ length: 30 }).map((_, i) => (
                <div
                  key={i}
                  className="bg-on-surface-variant/30 rounded-sm"
                  style={{
                    width: i % 3 === 0 ? "3px" : "1.5px",
                    height: "100%",
                  }}
                />
              ))}
            </div>
            <span className="text-[10px] text-on-surface-variant font-label tracking-widest uppercase">
              {t('inventory_detail.barcode')}
            </span>
            <span className="text-xs text-on-surface font-label tabular-nums tracking-wider">
              {ASSET.serialNumber}
            </span>
          </div>

          <div className="mt-auto pt-4 flex items-center gap-2">
            <Icon name="nfc" className="text-primary text-lg" />
            <span className="text-xs text-on-surface-variant font-label">
              {t('inventory_detail.nfc_tag')}: <span className="text-[#69db7c]">{t('inventory_detail.detected')}</span>
            </span>
          </div>
        </div>
      </div>

      {/* Scan History + Discrepancy Notes */}
      <div className="grid grid-cols-2 gap-4">
        {/* Scan History Timeline */}
        <div className="bg-surface-container rounded-xl p-5">
          <div className="flex items-center gap-2 mb-4">
            <Icon name="history" className="text-primary text-xl" />
            <h2 className="font-headline text-base font-bold text-on-surface">
              {t('inventory_detail.scan_history')}
            </h2>
            <span className="ml-auto text-[10px] text-on-surface-variant font-label">
              {SCAN_HISTORY.length} {t('common.records')}
            </span>
          </div>

          <div className="flex flex-col">
            {SCAN_HISTORY.map((scan, i) => {
              const isLast = i === SCAN_HISTORY.length - 1;
              let dotColor = "bg-[#69db7c]";
              let icon = "check_circle";
              if (scan.result === "status_mismatch") {
                dotColor = "bg-error";
                icon = "error";
              } else if (scan.result === "location_update") {
                dotColor = "bg-primary";
                icon = "swap_horiz";
              }

              return (
                <div key={i} className="flex gap-4">
                  {/* Timeline line */}
                  <div className="flex flex-col items-center">
                    <div
                      className={`w-8 h-8 rounded-lg flex items-center justify-center shrink-0 ${
                        scan.result === "status_mismatch"
                          ? "bg-error-container"
                          : scan.result === "location_update"
                          ? "bg-primary-container"
                          : "bg-[#0a2e1a]"
                      }`}
                    >
                      <Icon
                        name={icon}
                        className={`text-base ${
                          scan.result === "status_mismatch"
                            ? "text-error"
                            : scan.result === "location_update"
                            ? "text-primary"
                            : "text-[#69db7c]"
                        }`}
                      />
                    </div>
                    {!isLast && (
                      <div className="w-px h-full bg-surface-container-high min-h-[20px]" />
                    )}
                  </div>

                  {/* Content */}
                  <div className="pb-5 flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <span className="text-xs font-headline font-bold text-on-surface tabular-nums">
                        {scan.timestamp}
                      </span>
                      <span className="text-[10px] text-on-surface-variant font-label">
                        {t('inventory_detail.via')} {scan.method}
                      </span>
                    </div>
                    <p className="text-xs text-on-surface-variant font-label mb-1">
                      Operator: {scan.operator}
                    </p>
                    <p className="text-sm text-on-surface leading-relaxed">
                      {scan.note}
                    </p>
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* Discrepancy Notes */}
        <div className="bg-surface-container rounded-xl p-5">
          <div className="flex items-center gap-2 mb-4">
            <Icon name="assignment_late" className="text-error text-xl" />
            <h2 className="font-headline text-base font-bold text-on-surface">
              {t('inventory_detail.discrepancy_notes')}
            </h2>
            <span className="ml-auto text-[10px] text-error font-label font-bold">
              {DISCREPANCY_NOTES.length} entries
            </span>
          </div>

          <div className="flex flex-col gap-3">
            {DISCREPANCY_NOTES.map((note) => (
              <div
                key={note.id}
                className="bg-surface-container-low rounded-xl p-4"
              >
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-2">
                    <span className="text-xs font-headline font-bold text-on-surface">
                      {note.id}
                    </span>
                    <span
                      className={`text-[10px] font-label font-bold tracking-widest px-2 py-0.5 rounded-lg ${
                        note.severity === "HIGH"
                          ? "bg-error-container text-error"
                          : "bg-primary-container text-primary"
                      }`}
                    >
                      {note.severity}
                    </span>
                  </div>
                  <span className="text-[10px] text-on-surface-variant font-label tabular-nums">
                    {note.timestamp}
                  </span>
                </div>
                <p className="text-xs text-on-surface-variant font-label mb-2">
                  Author: {note.author}
                </p>
                <p className="text-sm text-on-surface leading-relaxed">
                  {note.text}
                </p>
              </div>
            ))}
          </div>

          {/* Add note */}
          <div className="mt-4 bg-surface-container-low rounded-xl p-4">
            <div className="flex items-center gap-2 mb-3">
              <Icon name="add_notes" className="text-primary text-lg" />
              <span className="text-xs font-label text-on-surface-variant uppercase tracking-widest">
                {t('inventory_detail.add_discrepancy_note')}
              </span>
            </div>
            <div className="bg-surface-container rounded-xl p-3 min-h-[60px] flex items-start">
              <span className="text-sm text-on-surface-variant/50">
                {t('inventory_detail.enter_observation_details')}
              </span>
            </div>
            <div className="flex items-center justify-between mt-3">
              <div className="flex items-center gap-3">
                <button className="text-xs text-on-surface-variant font-label flex items-center gap-1 hover:text-primary transition-colors">
                  <Icon name="attach_file" className="text-sm" />
                  {t('inventory_detail.attach')}
                </button>
                <button className="text-xs text-on-surface-variant font-label flex items-center gap-1 hover:text-primary transition-colors">
                  <Icon name="photo_camera" className="text-sm" />
                  {t('inventory_detail.photo')}
                </button>
              </div>
              <button className="bg-primary hover:opacity-90 text-on-primary px-4 py-1.5 rounded-lg text-xs font-label font-bold transition-opacity">
                {t('inventory_detail.submit_note')}
              </button>
            </div>
          </div>

          {/* Action buttons */}
          <div className="mt-4 flex items-center gap-2">
            <button className="flex-1 bg-[#0a2e1a] hover:opacity-90 text-[#69db7c] px-4 py-2.5 rounded-xl text-xs font-label font-bold flex items-center justify-center gap-2 transition-opacity">
              <Icon name="check_circle" className="text-lg" />
              {t('inventory_detail.mark_resolved')}
            </button>
            <button className="flex-1 bg-error-container hover:opacity-90 text-error px-4 py-2.5 rounded-xl text-xs font-label font-bold flex items-center justify-center gap-2 transition-opacity">
              <Icon name="escalator_warning" className="text-lg" />
              {t('inventory_detail.escalate')}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
});

export default InventoryItemDetail;
