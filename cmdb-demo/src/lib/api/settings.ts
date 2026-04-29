import { apiClient } from './client'
import type { ApiResponse } from './types'

// Manually typed (additive) — backend openapi.yaml defines schema
// AssetLifespanConfig (W3.2-backend). We don't regen api-types.ts here
// to avoid cascading drift in unrelated modules (e.g. prediction.ts).
export interface AssetLifespanConfig {
  server?: number
  network?: number
  storage?: number
  power?: number
}

// Canonical defaults — must mirror backend impl_prediction_upgrades.go.
// Keep in sync if backend defaults shift.
export const ASSET_LIFESPAN_DEFAULTS: Required<AssetLifespanConfig> = {
  server: 5,
  network: 7,
  storage: 5,
  power: 10,
}

export const ASSET_LIFESPAN_MIN = 1
export const ASSET_LIFESPAN_MAX = 30

export const settingsApi = {
  getAssetLifespan: () =>
    apiClient.get<ApiResponse<AssetLifespanConfig>>('/settings/asset-lifespan'),
  updateAssetLifespan: (data: AssetLifespanConfig) =>
    apiClient.put<ApiResponse<AssetLifespanConfig>>('/settings/asset-lifespan', data),
}
