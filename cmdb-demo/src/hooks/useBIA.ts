import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { biaApi } from '../lib/api/bia'

export function useBIAAssessments(params?: any) {
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
    mutationFn: (data: any) => biaApi.createAssessment(data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['biaAssessments'] })
      qc.invalidateQueries({ queryKey: ['biaStats'] })
    },
  })
}

export function useUpdateBIAAssessment() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: any }) =>
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
    mutationFn: ({ id, data }: { id: string; data: any }) =>
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
      data: any
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
