import { useEffect, useRef, useCallback } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useAuthStore } from '../stores/authStore'

const WS_RECONNECT_DELAY = 3000

export function useWebSocket() {
  const token = useAuthStore((s) => s.accessToken)
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const queryClient = useQueryClient()
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout>>()

  const invalidateByEvent = useCallback(
    (type: string) => {
      switch (type) {
        case 'asset.created':
        case 'asset.updated':
        case 'asset.deleted':
          queryClient.invalidateQueries({ queryKey: ['assets'] })
          queryClient.invalidateQueries({ queryKey: ['dashboardStats'] })
          break
        case 'alert.fired':
        case 'alert.resolved':
          queryClient.invalidateQueries({ queryKey: ['alerts'] })
          queryClient.invalidateQueries({ queryKey: ['alertRules'] })
          queryClient.invalidateQueries({ queryKey: ['incidents'] })
          queryClient.invalidateQueries({ queryKey: ['dashboardStats'] })
          break
        case 'maintenance.order_created':
        case 'maintenance.order_transitioned':
          queryClient.invalidateQueries({ queryKey: ['workOrders'] })
          queryClient.invalidateQueries({ queryKey: ['dashboardStats'] })
          break
        case 'prediction.created':
          queryClient.invalidateQueries({ queryKey: ['predictions'] })
          queryClient.invalidateQueries({ queryKey: ['rcaAnalyses'] })
          break
      }
    },
    [queryClient]
  )

  const connect = useCallback(() => {
    if (!token || !isAuthenticated) return

    // Derive WS URL from API URL
    const apiUrl = import.meta.env.VITE_API_URL || '/api/v1'
    const wsProtocol = apiUrl.startsWith('https') ? 'wss' : 'ws'
    const baseUrl = apiUrl.replace(/^https?/, wsProtocol)
    const wsUrl = `${baseUrl}/ws`

    const ws = new WebSocket(wsUrl, [`access_token.${token}`])
    wsRef.current = ws

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        if (msg.type) {
          invalidateByEvent(msg.type)
        }
      } catch {
        // ignore malformed messages
      }
    }

    ws.onclose = () => {
      wsRef.current = null
      // Auto-reconnect if still authenticated
      if (useAuthStore.getState().isAuthenticated) {
        reconnectTimerRef.current = setTimeout(connect, WS_RECONNECT_DELAY)
      }
    }

    ws.onerror = () => {
      ws.close()
    }
  }, [token, isAuthenticated, invalidateByEvent])

  useEffect(() => {
    connect()

    return () => {
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current)
      }
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
    }
  }, [connect])
}
