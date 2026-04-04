import { apiClient } from './client'
import type { ApiResponse } from './types'
import type { components } from '../../generated/api-types'

export type PredictionResult = components['schemas']['PredictionResult']
export type RCAAnalysis = components['schemas']['RCAAnalysis']

export const predictionApi = {
  listModels: () =>
    apiClient.get<ApiResponse<any[]>>('/prediction/models'),
  listByCI: (ciID: string) =>
    apiClient.get<ApiResponse<PredictionResult[]>>(`/prediction/results/ci/${ciID}`),
  createPrediction: (data: any) =>
    apiClient.post<ApiResponse<PredictionResult>>('/prediction/results', data),
  createRCA: (data: any) =>
    apiClient.post<ApiResponse<RCAAnalysis>>('/prediction/rca', data),
  verifyRCA: (id: string, data: any) =>
    apiClient.post<ApiResponse<RCAAnalysis>>(`/prediction/rca/${id}/verify`, data),
}
