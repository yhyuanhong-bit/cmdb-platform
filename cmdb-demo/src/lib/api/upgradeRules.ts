import { apiClient } from './client'
import type { ApiResponse } from './types'

export interface UpgradeRule {
  id: string
  asset_type: string
  category: string
  metric_name: string
  threshold: number
  duration_days: number
  priority: string
  recommendation: string
  enabled: boolean
}

export type CreateUpgradeRuleInput = Omit<UpgradeRule, 'id'>
export type UpdateUpgradeRuleInput = Partial<Omit<UpgradeRule, 'id' | 'asset_type' | 'category' | 'metric_name'>>

export const upgradeRulesApi = {
  list: () =>
    apiClient.get<ApiResponse<{ rules: UpgradeRule[] }>>('/upgrade-rules'),
  create: (data: CreateUpgradeRuleInput) =>
    apiClient.post<ApiResponse<{ id: string }>>('/upgrade-rules', data),
  update: (id: string, data: UpdateUpgradeRuleInput) =>
    apiClient.put<ApiResponse<{ updated: boolean }>>(`/upgrade-rules/${id}`, data),
  delete: (id: string) =>
    apiClient.del(`/upgrade-rules/${id}`),
}
