/* ──────────────────────────────────────────────
   Fallback / seed data for AlertTopologyAnalysis
   Extracted from src/pages/AlertTopologyAnalysis.tsx
   ────────────────────────────────────────────── */

export const FALLBACK_ALERTS = [
  { id: 'ALT-001', severity: 'CRITICAL' as const, assetName: 'DB-PROD-SQL-01', description: 'CPU utilization exceeded 85% threshold for over 15 minutes.', timestamp: '2026-03-28 09:14:22', nodeId: 'node-1' },
  { id: 'ALT-002', severity: 'WARNING' as const, assetName: 'APP-PORTAL-WEB-04', description: 'HTTP response time degraded to 4.2s (SLA threshold: 2s).', timestamp: '2026-03-28 09:15:41', nodeId: 'node-2' },
]
