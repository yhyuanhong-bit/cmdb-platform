import { apiClient } from './client'

export const activityApi = {
  getFeed: (params: Record<string, string>) =>
    apiClient.get('/activity-feed', params),
}
