import { apiClient } from './client'

export const notificationApi = {
  list: () => apiClient.get('/notifications'),
  count: () => apiClient.get('/notifications/count'),
  markRead: (id: string) => apiClient.post(`/notifications/${id}/read`),
  markAllRead: () => apiClient.post('/notifications/read-all'),
}
