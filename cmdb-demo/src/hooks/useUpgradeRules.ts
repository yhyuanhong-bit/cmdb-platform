import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { upgradeRulesApi } from '../lib/api/upgradeRules'
import type { CreateUpgradeRuleInput, UpdateUpgradeRuleInput } from '../lib/api/upgradeRules'

export function useUpgradeRules() {
  return useQuery({
    queryKey: ['upgradeRules'],
    queryFn: () => upgradeRulesApi.list(),
  })
}

export function useCreateUpgradeRule() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: CreateUpgradeRuleInput) => upgradeRulesApi.create(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['upgradeRules'] }),
  })
}

export function useUpdateUpgradeRule() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateUpgradeRuleInput }) =>
      upgradeRulesApi.update(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['upgradeRules'] }),
  })
}

export function useDeleteUpgradeRule() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => upgradeRulesApi.delete(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['upgradeRules'] }),
  })
}
