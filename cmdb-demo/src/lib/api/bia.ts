import { apiClient } from './client'

export const biaApi = {
  listAssessments: (params?: any) =>
    apiClient.get('/bia/assessments', params),
  getAssessment: (id: string) =>
    apiClient.get(`/bia/assessments/${id}`),
  createAssessment: (data: any) =>
    apiClient.post('/bia/assessments', data),
  updateAssessment: (id: string, data: any) =>
    apiClient.put(`/bia/assessments/${id}`, data),
  deleteAssessment: (id: string) =>
    apiClient.del(`/bia/assessments/${id}`),
  listRules: () =>
    apiClient.get('/bia/rules'),
  updateRule: (id: string, data: any) =>
    apiClient.put(`/bia/rules/${id}`, data),
  listDependencies: (assessmentId: string) =>
    apiClient.get(`/bia/assessments/${assessmentId}/dependencies`),
  createDependency: (assessmentId: string, data: any) =>
    apiClient.post(`/bia/assessments/${assessmentId}/dependencies`, data),
  getStats: () =>
    apiClient.get('/bia/stats'),
}
