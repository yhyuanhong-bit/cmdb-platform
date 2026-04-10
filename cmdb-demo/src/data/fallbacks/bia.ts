/* ──────────────────────────────────────────────
   Fallback / seed data for BIAOverview
   Extracted from src/pages/bia/BIAOverview.tsx
   ────────────────────────────────────────────── */

export const SEED_RULES = [
  { id: '1', tier_name: 'critical',  tier_level: 1, display_name: 'Tier 1 - CRITICAL',  description: 'Core payment systems, building monitoring - downtime causes major financial/safety impact', color: '#ff6b6b', icon: 'error', min_score: 85, max_score: 100, rto_threshold: 4, rpo_threshold: 15 },
  { id: '2', tier_name: 'important', tier_level: 2, display_name: 'Tier 2 - IMPORTANT', description: 'Core system groups (CRM, ERP) - downtime impacts business efficiency', color: '#ffa94d', icon: 'warning', min_score: 60, max_score: 84, rto_threshold: 12, rpo_threshold: 60 },
  { id: '3', tier_name: 'normal',    tier_level: 3, display_name: 'Tier 3 - NORMAL',    description: 'General business systems - downtime has workarounds available', color: '#9ecaff', icon: 'info', min_score: 30, max_score: 59, rto_threshold: 24, rpo_threshold: 240 },
  { id: '4', tier_name: 'minor',     tier_level: 4, display_name: 'Tier 4 - MINOR',     description: 'Test/sandbox environments - downtime has no business impact', color: '#8e9196', icon: 'expand_circle_down', min_score: 0, max_score: 29, rto_threshold: 72, rpo_threshold: null },
]

export const SEED_ASSESSMENTS = [
  { id: '1', system_name: 'Core Payment Gateway', system_code: 'SYS-PROD-PAY-001', owner: 'David Yun', bia_score: 98, tier: 'critical',  rto_hours: 4, rpo_minutes: 15,   data_compliance: true,  asset_compliance: true,  audit_compliance: true,  description: 'Processes all online payment transactions' },
  { id: '2', system_name: 'CRM Core',             system_code: 'SYS-PROD-CRM-001', owner: 'Lin Sheng', bia_score: 85, tier: 'important', rto_hours: 12, rpo_minutes: 120, data_compliance: true,  asset_compliance: true,  audit_compliance: false, description: 'Customer data and service history' },
  { id: '3', system_name: 'Admin Panel',           system_code: 'SYS-CORP-ADM-001', owner: 'Wang Zhi', bia_score: 62, tier: 'normal',    rto_hours: 24, rpo_minutes: 240, data_compliance: true,  asset_compliance: false, audit_compliance: false, description: 'Internal process management' },
  { id: '4', system_name: 'QA Sandbox',            system_code: 'SYS-TEST-QA-001',  owner: null,       bia_score: 15, tier: 'minor',     rto_hours: 72, rpo_minutes: null, data_compliance: false, asset_compliance: false, audit_compliance: false, description: 'QA testing environment' },
]

export const SEED_STATS = {
  total: 4,
  by_tier: { critical: 1, important: 1, normal: 1, minor: 1 },
  avg_compliance: 58,
  data_compliant: 3,
  asset_compliant: 2,
  audit_compliant: 1,
}
