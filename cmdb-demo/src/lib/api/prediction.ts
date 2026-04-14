import { apiClient } from './client'
import type { ApiResponse } from './types'
import type { components } from '../../generated/api-types'

export type PredictionResult = components['schemas']['PredictionResult']
export type RCAAnalysis = components['schemas']['RCAAnalysis']

export interface PredictionModel {
  id: string
  name: string
  type: string
  version: string
  accuracy?: number
  last_trained?: string
  enabled?: boolean
}

export interface CreateRCAData {
  ci_id?: string
  incident_id?: string
  model_name?: string
  context?: string
  alert_id?: string
  description?: string
  symptoms?: string[]
  metadata?: Record<string, unknown>
}

export interface VerifyRCAData {
  verified?: boolean
  verified_by?: string
  root_cause?: string
  resolution?: string
  notes?: string
  [key: string]: unknown
}

export const predictionApi = {
  listModels: () =>
    apiClient.get<ApiResponse<PredictionModel[]>>('/prediction/models'),
  listByCI: (ciID: string) =>
    apiClient.get<ApiResponse<PredictionResult[]>>(`/prediction/results/ci/${ciID}`),
  createRCA: (data: CreateRCAData) =>
    apiClient.post<ApiResponse<RCAAnalysis>>('/prediction/rca', data),
  verifyRCA: (id: string, data: VerifyRCAData) =>
    apiClient.post<ApiResponse<RCAAnalysis>>(`/prediction/rca/${id}/verify`, data),
  getRUL: (assetId: string) =>
    apiClient.get(`/prediction/rul/${assetId}`),
  getFailureDistribution: () =>
    apiClient.get('/prediction/failure-distribution'),
}
