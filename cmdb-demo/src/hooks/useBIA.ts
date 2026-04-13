import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { biaApi } from '../lib/api/bia'

export function useBIAAssessments(params?: Record<string, string>) {
  return useQuery({
    queryKey: ['biaAssessments', params],
    queryFn: () => biaApi.listAssessments(params),
  })
}

export function useBIAAssessment(id: string) {
  return useQuery({
    queryKey: ['biaAssessments', id],
    queryFn: () => biaApi.getAssessment(id),
    enabled: !!id,
  })
}

export function useCreateBIAAssessment() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: Record<string, unknown>) => biaApi.createAssessment(data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['biaAssessments'] })
      qc.invalidateQueries({ queryKey: ['biaStats'] })
    },
  })
}

export function useUpdateBIAAssessment() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Record<string, unknown> }) =>
      biaApi.updateAssessment(id, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['biaAssessments'] })
    },
  })
}

export function useDeleteBIAAssessment() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => biaApi.deleteAssessment(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['biaAssessments'] })
      qc.invalidateQueries({ queryKey: ['biaStats'] })
    },
  })
}

export function useBIAScoringRules() {
  return useQuery({
    queryKey: ['biaScoringRules'],
    queryFn: () => biaApi.listRules(),
  })
}

export function useUpdateBIAScoringRule() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Record<string, unknown> }) =>
      biaApi.updateRule(id, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['biaScoringRules'] })
    },
  })
}

export function useBIADependencies(assessmentId: string) {
  return useQuery({
    queryKey: ['biaDependencies', assessmentId],
    queryFn: () => biaApi.listDependencies(assessmentId),
    enabled: !!assessmentId,
  })
}

export function useCreateBIADependency() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({
      assessmentId,
      data,
    }: {
      assessmentId: string
      data: Record<string, unknown>
    }) => biaApi.createDependency(assessmentId, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['biaDependencies'] })
    },
  })
}

export function useBIAStats() {
  return useQuery({
    queryKey: ['biaStats'],
    queryFn: () => biaApi.getStats(),
  })
}

export function useBIAImpact(assetId: string) {
  return useQuery({
    queryKey: ['biaImpact', assetId],
    queryFn: () => biaApi.getImpact(assetId),
    enabled: !!assetId,
  })
}
