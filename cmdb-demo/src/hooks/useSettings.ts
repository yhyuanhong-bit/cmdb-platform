import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  settingsApi,
  type AssetLifespanConfig,
} from '../lib/api/settings'

const ASSET_LIFESPAN_KEY = ['settings', 'asset-lifespan'] as const

export function useAssetLifespanSettings() {
  return useQuery({
    queryKey: ASSET_LIFESPAN_KEY,
    queryFn: settingsApi.getAssetLifespan,
    // Tenant settings rarely change; avoid hammering the endpoint.
    staleTime: 60_000,
  })
}

export function useUpdateAssetLifespan() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: AssetLifespanConfig) => settingsApi.updateAssetLifespan(data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ASSET_LIFESPAN_KEY })
    },
  })
}
