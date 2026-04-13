import { apiClient } from './client'
import type { ApiResponse } from './types'

export interface SyncState {
  node_id: string
  entity_type: string
  last_sync_version: number
  last_sync_at: string
  status: string
  error_message: string | null
}

export interface SyncConflict {
  id: string
  entity_type: string
  entity_id: string
  local_version: number
  remote_version: number
  local_diff: Record<string, unknown>
  remote_diff: Record<string, unknown>
  created_at: string
}

export interface SyncNodeGap {
  node_id: string
  last_sync_version: number
  gap: number
}

export interface SyncEntityStats {
  entity_type: string
  max_version: number
  nodes: SyncNodeGap[]
}

export const syncApi = {
  getState: () =>
    apiClient.get<ApiResponse<SyncState[]>>('/sync/state'),
  getConflicts: () =>
    apiClient.get<ApiResponse<SyncConflict[]>>('/sync/conflicts'),
  resolveConflict: (id: string, resolution: 'local_wins' | 'remote_wins') =>
    apiClient.post<void>(`/sync/conflicts/${id}/resolve`, { resolution }),
  getStats: () =>
    apiClient.get<ApiResponse<SyncEntityStats[]>>('/sync/stats'),
}
