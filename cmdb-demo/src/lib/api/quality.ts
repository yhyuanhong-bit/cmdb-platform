import { apiClient } from './client'
import type { ApiResponse } from './types'

export interface QualityRule {
  id: string
  dimension: string
  field_name: string
  rule_type: string
  weight: number
  enabled: boolean
}

export interface DimensionScore {
  dimension: string
  score: number
  weight: number
}

export interface WorstAsset {
  asset_id?: string
  asset_tag: string
  asset_name: string
  total_score: number
  issues: number
}

export interface QualityDashboardData {
  total_score: number
  dimensions: DimensionScore[]
  worst_assets: WorstAsset[]
}

export interface QualityHistoryPoint {
  recorded_at: string
  total_score: number
  dimensions?: DimensionScore[]
}

export interface CreateQualityRuleData {
  name?: string
  field?: string
  field_name?: string
  dimension?: string
  rule_type?: string
  severity?: string
  weight?: number
  condition?: Record<string, unknown>
  enabled?: boolean
  [key: string]: unknown
}

export const qualityApi = {
  listRules: () => apiClient.get<ApiResponse<QualityRule[]>>('/quality/rules'),
  createRule: (data: CreateQualityRuleData) => apiClient.post<ApiResponse<QualityRule>>('/quality/rules', data),
  getDashboard: () => apiClient.get<ApiResponse<QualityDashboardData>>('/quality/dashboard'),
  getAssetHistory: (assetId: string) => apiClient.get<ApiResponse<QualityHistoryPoint[]>>(`/quality/history/${assetId}`),
  triggerScan: () => apiClient.post<ApiResponse<{ scanned: number; updated: number }>>('/quality/scan', {}),
}
