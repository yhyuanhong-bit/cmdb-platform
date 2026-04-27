import { apiClient } from './client'
import type { ApiResponse } from './types'

/** Energy tariff — $/kWh per (tenant, location) over a date range.
 *  Money values cross the wire as decimal strings to avoid float drift on
 *  multi-asset totals. The UI parses these on render only. */
export interface EnergyTariff {
  id: string
  location_id?: string | null
  currency: string
  rate_per_kwh: string
  effective_from: string // ISO date YYYY-MM-DD
  effective_to?: string | null
  notes?: string | null
  created_at: string
  updated_at?: string | null
}

export interface CreateTariffInput {
  location_id?: string | null
  currency?: string
  rate_per_kwh: string
  effective_from: string
  effective_to?: string | null
  notes?: string
}

export interface UpdateTariffInput {
  currency?: string
  rate_per_kwh?: string
  effective_from?: string
  effective_to?: string | null
  clear_effective_to?: boolean
  notes?: string
}

export interface EnergyDailyKwh {
  asset_id: string
  asset_tag?: string | null
  asset_name?: string | null
  location_id?: string | null
  day: string
  kwh_total: string
  kw_peak: string
  kw_avg: string
  sample_count: number
  computed_at?: string | null
}

export interface EnergyBillLine {
  asset_id: string
  location_id?: string | null
  kwh: string
  rate_per_kwh: string
  cost: string
  currency: string
}

export interface EnergyBill {
  day_from: string
  day_to: string
  total_kwh: string
  total_cost: string
  currency: string
  /** True when assets in the window span tariffs in different currencies.
   *  When true, the UI must NOT render total_cost as a single value. */
  currency_mixed: boolean
  lines?: EnergyBillLine[]
}

/* ------------------------------------------------------------------ */
/*  Wave 6.2: PUE + anomaly types                                      */
/* ------------------------------------------------------------------ */

/** Per-(location, day) PUE row. PUE is null when there's no IT load
 *  recorded — the UI should display "—" rather than ∞. */
export interface EnergyLocationPue {
  location_id: string
  location_name?: string | null
  location_level?: string | null
  day: string
  it_kwh: string
  non_it_kwh: string
  total_kwh: string
  it_asset_count: number
  non_it_asset_count: number
  pue?: string | null
  computed_at?: string | null
}

/** Asset-day anomaly. Score = observed / baseline_median; >=1 for
 *  'high', between 0 and 1 for 'low'. */
export type EnergyAnomalyKind = 'high' | 'low'
export type EnergyAnomalyStatus = 'open' | 'ack' | 'resolved'

export interface EnergyAnomaly {
  asset_id: string
  asset_tag?: string | null
  asset_name?: string | null
  location_id?: string | null
  day: string
  kind: EnergyAnomalyKind
  baseline_median: string
  observed_kwh: string
  score: string
  status: EnergyAnomalyStatus
  detected_at: string
  reviewed_by?: string | null
  reviewed_at?: string | null
  note?: string | null
}

export interface ListAnomaliesParams {
  page?: number
  page_size?: number
  status?: EnergyAnomalyStatus
  day_from: string
  day_to: string
}

function anomalyParams(p: ListAnomaliesParams): Record<string, string> {
  const out: Record<string, string> = {
    day_from: p.day_from,
    day_to: p.day_to,
  }
  if (p.page != null) out.page = String(p.page)
  if (p.page_size != null) out.page_size = String(p.page_size)
  if (p.status) out.status = p.status
  return out
}

export const energyBillingApi = {
  // Tariffs.
  listTariffs: () =>
    apiClient.get<ApiResponse<EnergyTariff[]>>('/energy/billing/tariffs'),
  getTariff: (id: string) =>
    apiClient.get<ApiResponse<EnergyTariff>>(`/energy/billing/tariffs/${id}`),
  createTariff: (body: CreateTariffInput) =>
    apiClient.post<ApiResponse<EnergyTariff>>('/energy/billing/tariffs', body),
  updateTariff: (id: string, body: UpdateTariffInput) =>
    apiClient.put<ApiResponse<EnergyTariff>>(`/energy/billing/tariffs/${id}`, body),
  deleteTariff: (id: string) =>
    apiClient.del(`/energy/billing/tariffs/${id}`),

  // Aggregator (idempotent backfill for a date range).
  aggregateRange: (dayFrom: string, dayTo: string) =>
    apiClient.post<ApiResponse<{ aggregated_count: number }>>('/energy/billing/aggregate', {
      day_from: dayFrom,
      day_to: dayTo,
    }),

  // Daily rollup query.
  listDaily: (dayFrom: string, dayTo: string) =>
    apiClient.get<ApiResponse<EnergyDailyKwh[]>>('/energy/billing/daily', {
      day_from: dayFrom,
      day_to: dayTo,
    }),

  // Bill calculator.
  getBill: (dayFrom: string, dayTo: string) =>
    apiClient.get<ApiResponse<EnergyBill>>('/energy/billing/bill', {
      day_from: dayFrom,
      day_to: dayTo,
    }),

  // Wave 6.2: PUE + anomalies.
  listPue: (dayFrom: string, dayTo: string, locationId?: string) => {
    const params: Record<string, string> = { day_from: dayFrom, day_to: dayTo }
    if (locationId) params.location_id = locationId
    return apiClient.get<ApiResponse<EnergyLocationPue[]>>('/energy/pue', params)
  },
  listAnomalies: (params: ListAnomaliesParams) =>
    apiClient.get<{ data: EnergyAnomaly[]; pagination?: { total: number; page: number; page_size: number } }>(
      '/energy/anomalies',
      anomalyParams(params),
    ),
  transitionAnomaly: (assetId: string, day: string, status: EnergyAnomalyStatus, note?: string) =>
    apiClient.post<ApiResponse<EnergyAnomaly>>(
      `/energy/anomalies/${assetId}/${day}/transition`,
      { status, note: note ?? '' },
    ),
}
