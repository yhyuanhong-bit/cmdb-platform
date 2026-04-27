import { apiClient } from './client'
import type { ApiResponse } from './types'

export type SchedulerStatus = 'ok' | 'lagging' | 'stale' | 'never_ticked'

export interface SchedulerHealth {
  name: string
  expected_interval_seconds?: number | null
  last_tick_at?: string | null
  seconds_since_tick?: number | null
  status: SchedulerStatus
}

export interface SchedulerHealthReport {
  all_healthy: boolean
  schedulers: SchedulerHealth[]
}

export const schedulerHealthApi = {
  get: () => apiClient.get<ApiResponse<SchedulerHealthReport>>('/admin/scheduler-health'),
}
