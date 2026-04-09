import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { sensorApi } from '../lib/api/sensors'

const DEFAULT_TENANT = 'a0000000-0000-0000-0000-000000000001'

export function useSensors() {
  return useQuery({
    queryKey: ['sensors'],
    queryFn: () => sensorApi.list({ tenant_id: DEFAULT_TENANT }),
  })
}

export function useCreateSensor() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: sensorApi.create,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['sensors'] }),
  })
}

export function useUpdateSensor() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Record<string, unknown> }) => sensorApi.update(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['sensors'] }),
  })
}

export function useDeleteSensor() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => sensorApi.delete(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['sensors'] }),
  })
}
