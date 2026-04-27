import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  energyBillingApi,
  type EnergyAnomalyStatus,
  type ListAnomaliesParams,
} from '../lib/api/energyBilling'

/* ------------------------------------------------------------------ */
/*  PUE                                                                */
/* ------------------------------------------------------------------ */

export function useEnergyPue(
  dayFrom: string | undefined,
  dayTo: string | undefined,
  locationId?: string,
) {
  return useQuery({
    queryKey: ['energy', 'pue', dayFrom, dayTo, locationId ?? null],
    queryFn: () => energyBillingApi.listPue(dayFrom!, dayTo!, locationId),
    enabled: !!dayFrom && !!dayTo,
  })
}

/* ------------------------------------------------------------------ */
/*  Anomalies                                                          */
/* ------------------------------------------------------------------ */

export function useEnergyAnomalies(params: ListAnomaliesParams | null) {
  return useQuery({
    queryKey: ['energy', 'anomalies', params],
    queryFn: () => energyBillingApi.listAnomalies(params!),
    enabled: !!params,
  })
}

export function useTransitionAnomaly() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({
      assetId,
      day,
      status,
      note,
    }: {
      assetId: string
      day: string
      status: EnergyAnomalyStatus
      note?: string
    }) => energyBillingApi.transitionAnomaly(assetId, day, status, note),
    onSuccess: () => {
      // Invalidate every anomaly list — status filters mean a single row
      // moves between several views and we'd rather refetch all than
      // chase the right key.
      qc.invalidateQueries({ queryKey: ['energy', 'anomalies'] })
    },
  })
}
