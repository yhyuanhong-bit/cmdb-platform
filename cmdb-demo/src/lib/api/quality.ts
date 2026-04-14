import { apiClient } from './client'

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
  listRules: () => apiClient.get('/quality/rules'),
  createRule: (data: CreateQualityRuleData) => apiClient.post('/quality/rules', data),
  getDashboard: () => apiClient.get('/quality/dashboard'),
  getAssetHistory: (assetId: string) => apiClient.get(`/quality/history/${assetId}`),
  triggerScan: () => apiClient.post('/quality/scan', {}),
}
