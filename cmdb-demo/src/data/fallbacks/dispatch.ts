/* ──────────────────────────────────────────────
   Fallback / seed data for TaskDispatch
   Extracted from src/pages/TaskDispatch.tsx
   ────────────────────────────────────────────── */

export interface ZoneData {
  label: string
  pct: number
  color: string
}

export function FALLBACK_ZONES(colors: string[]): ZoneData[] {
  return [
    { label: 'Zone A', pct: 0, color: colors[0] },
    { label: 'Zone B', pct: 0, color: colors[1] },
    { label: 'Zone C', pct: 0, color: colors[2] },
    { label: 'Zone D', pct: 0, color: colors[3] },
  ]
}
