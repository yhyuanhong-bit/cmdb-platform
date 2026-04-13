/* ──────────────────────────────────────────────
   Fallback / seed data for AssetLifecycleTimeline
   Extracted from src/pages/AssetLifecycleTimeline.tsx
   ────────────────────────────────────────────── */

export const ASSET_FALLBACK = {
  id: '-',
  status: 'UNKNOWN',
  lastSync: '-',
  serial: '-',
  primaryIp: '-',
  avgLatency: '-',
  uptime: '-',
}

export const TIMELINE_STAGES = [
  {
    id: 1,
    phaseKey: 'asset_lifecycle_timeline.phase_procurement',
    statusKey: 'asset_lifecycle_timeline.status_completed',
    statusColor: 'text-[#69db7c]',
    dotColor: 'bg-[#69db7c]',
    lineColor: 'bg-[#69db7c]/40',
    date: '2023.01.12',
    technician: 'M. Sterling',
    description:
      'Approved vendor bid accepted. Hardware order placed with Dell Technologies under PO-2023-8842.',
    hasDetail: true,
  },
  {
    id: 2,
    phaseKey: 'asset_lifecycle_timeline.phase_installation_deployment',
    statusKey: 'asset_lifecycle_timeline.status_completed',
    statusColor: 'text-[#69db7c]',
    dotColor: 'bg-[#69db7c]',
    lineColor: 'bg-[#69db7c]/40',
    date: '2023.01.20',
    technician: 'K. Vance',
    description:
      'Physical rack installation at IDC-NORTH-01. OS provisioning and network configuration completed.',
    hasDetail: true,
  },
  {
    id: 3,
    phaseKey: 'asset_lifecycle_timeline.phase_maintenance_cycle',
    statusKey: 'asset_lifecycle_timeline.status_ongoing',
    statusColor: 'text-[#ffa94d]',
    dotColor: 'bg-[#ffa94d]',
    lineColor: 'bg-[#ffa94d]/40',
    date: '2023.09.15',
    technician: 'J. Thorne',
    description:
      'Scheduled firmware update v4.2.1 applied. Cooling unit inspection flagged for follow-up.',
    hasDetail: false,
    highlighted: true,
  },
  {
    id: 4,
    phaseKey: 'asset_lifecycle_timeline.phase_infrastructure_upgrade',
    statusKey: 'asset_lifecycle_timeline.status_planned',
    statusColor: 'text-on-surface-variant',
    dotColor: 'bg-on-surface-variant/50',
    lineColor: 'bg-on-surface-variant/20',
    date: 'Expected: Q1 2024',
    technician: 'SysOps Alpha',
    description:
      'NVMe storage tier migration and memory expansion to 512GB. Budget allocated under CAPEX-2024-Q1.',
    hasDetail: false,
    costEstimate: '$14,200.00',
  },
  {
    id: 5,
    phaseKey: 'asset_lifecycle_timeline.phase_decommission_recycle',
    statusKey: 'asset_lifecycle_timeline.status_done',
    statusColor: 'text-on-surface-variant',
    dotColor: 'bg-on-surface-variant/50',
    lineColor: 'bg-transparent',
    date: '2027.01.12',
    technician: 'Standard 48-Mo Lifecycle',
    description:
      'End-of-life scheduled per corporate asset rotation policy. Data wipe and certified recycling.',
    hasDetail: false,
  },
]

export const FINANCIALS = {
  acquisitionCost: '$248,500.00',
  depreciatedValue: '$192,420.00',
  maintenanceRoi: 12.4,
}

export const COMPLIANCE = [
  {
    label: 'ISO 27001 CERTIFICATION',
    icon: 'verified',
    status: 'pass',
    color: 'text-[#69db7c]',
  },
  {
    label: 'SECURITY PATCHING V4.2',
    icon: 'security',
    status: 'pass',
    color: 'text-[#69db7c]',
  },
  {
    label: 'PHYSICAL ACCESS AUDIT',
    icon: 'warning',
    status: 'warn',
    color: 'text-error',
  },
]
