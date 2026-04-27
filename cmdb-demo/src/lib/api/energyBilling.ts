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
}
