import { apiClient } from './client'
import type { ApiResponse } from './types'

export type PredictiveRefreshKind =
  | 'warranty_expiring'
  | 'warranty_expired'
  | 'eol_approaching'
  | 'eol_passed'
  | 'aged_out'

export type PredictiveRefreshStatus = 'open' | 'ack' | 'resolved'

export interface PredictiveRefresh {
  asset_id: string
  asset_tag?: string | null
  asset_name?: string | null
  asset_type?: string | null
  location_id?: string | null
  kind: PredictiveRefreshKind
  /** 0-100 as decimal string. Higher = more urgent. */
  risk_score: string
  reason: string
  recommended_action?: string | null
  target_date?: string | null
  status: PredictiveRefreshStatus
  detected_at: string
  reviewed_by?: string | null
  reviewed_at?: string | null
  note?: string | null
  purchase_date?: string | null
  warranty_end?: string | null
  eol_date?: string | null
}

export interface ListPredictiveRefreshParams {
  page?: number
  page_size?: number
  status?: PredictiveRefreshStatus
  kind?: PredictiveRefreshKind
}

function listParams(p?: ListPredictiveRefreshParams): Record<string, string> | undefined {
  if (!p) return undefined
  const out: Record<string, string> = {}
  if (p.page != null) out.page = String(p.page)
  if (p.page_size != null) out.page_size = String(p.page_size)
  if (p.status) out.status = p.status
  if (p.kind) out.kind = p.kind
  return out
}

export const predictiveRefreshApi = {
  list: (p?: ListPredictiveRefreshParams) =>
    apiClient.get<{
      data: PredictiveRefresh[]
      pagination?: { total: number; page: number; page_size: number }
    }>('/predictive/refresh', listParams(p)),

  /** Manual rescan for the caller's tenant. The hourly scheduler runs
   *  the same code, so this endpoint is for "I just edited a warranty
   *  date and want to see the recommendation update now." */
  runScan: () =>
    apiClient.post<ApiResponse<{ assets_scanned: number; rows_upserted: number }>>(
      '/predictive/refresh/run',
    ),

  transition: (
    assetId: string,
    kind: PredictiveRefreshKind,
    status: PredictiveRefreshStatus,
    note?: string,
  ) =>
    apiClient.post<ApiResponse<PredictiveRefresh>>(
      `/predictive/refresh/${assetId}/${kind}/transition`,
      { status, note: note ?? '' },
    ),
}
