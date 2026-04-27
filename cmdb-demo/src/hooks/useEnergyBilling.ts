import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import {
  energyBillingApi,
  type CreateTariffInput,
  type UpdateTariffInput,
} from '../lib/api/energyBilling'

/** Invalidate every cached view that depends on tariffs. The bill view
 *  also depends because changing a tariff retroactively changes the cost
 *  number, so we drop those keys too. */
function invalidateTariffTree(qc: QueryClient) {
  qc.invalidateQueries({ queryKey: ['energy', 'tariffs'] })
  qc.invalidateQueries({ queryKey: ['energy', 'bill'] })
}

export function useEnergyTariffs() {
  return useQuery({
    queryKey: ['energy', 'tariffs'],
    queryFn: () => energyBillingApi.listTariffs(),
  })
}

export function useCreateEnergyTariff() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: CreateTariffInput) => energyBillingApi.createTariff(body),
    onSuccess: () => invalidateTariffTree(qc),
  })
}

export function useUpdateEnergyTariff() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: UpdateTariffInput }) =>
      energyBillingApi.updateTariff(id, body),
    onSuccess: () => invalidateTariffTree(qc),
  })
}

export function useDeleteEnergyTariff() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => energyBillingApi.deleteTariff(id),
    onSuccess: () => invalidateTariffTree(qc),
  })
}

export function useEnergyDaily(dayFrom: string | undefined, dayTo: string | undefined) {
  return useQuery({
    queryKey: ['energy', 'daily', dayFrom, dayTo],
    queryFn: () => energyBillingApi.listDaily(dayFrom!, dayTo!),
    enabled: !!dayFrom && !!dayTo,
  })
}

export function useEnergyBill(dayFrom: string | undefined, dayTo: string | undefined) {
  return useQuery({
    queryKey: ['energy', 'bill', dayFrom, dayTo],
    queryFn: () => energyBillingApi.getBill(dayFrom!, dayTo!),
    enabled: !!dayFrom && !!dayTo,
  })
}

export function useAggregateEnergyRange() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ dayFrom, dayTo }: { dayFrom: string; dayTo: string }) =>
      energyBillingApi.aggregateRange(dayFrom, dayTo),
    onSuccess: () => {
      // Aggregation produces fresh daily rows that drive the bill view.
      qc.invalidateQueries({ queryKey: ['energy', 'daily'] })
      qc.invalidateQueries({ queryKey: ['energy', 'bill'] })
    },
  })
}
