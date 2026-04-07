import { useQuery } from '@tanstack/react-query'
import { auditApi } from '../lib/api/audit'

export function useAuditEvents(params?: Record<string, string>) {
  return useQuery({
    queryKey: ['auditEvents', params],
    queryFn: () => auditApi.query(params),
  })
}

export function useAuditEventDetail(eventId: string) {
  return useQuery({
    queryKey: ['auditEventDetail', eventId],
    queryFn: () => auditApi.getEventById(eventId),
    enabled: !!eventId,
  })
}
