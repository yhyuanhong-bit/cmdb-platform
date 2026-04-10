import { useEffect, useRef, useCallback } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useAuthStore } from '../stores/authStore'

const WS_RECONNECT_DELAY = 3000
const WS_MAX_RETRIES = 5

export function useWebSocket() {
  const token = useAuthStore((s) => s.accessToken)
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const queryClient = useQueryClient()
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const retriesRef = useRef(0)

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
    let wsUrl: string
    if (apiUrl.startsWith('http')) {
      // Absolute URL: replace protocol
      const wsProtocol = apiUrl.startsWith('https') ? 'wss' : 'ws'
      wsUrl = apiUrl.replace(/^https?/, wsProtocol) + '/ws'
    } else {
      // Relative URL: build from current window location
      const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
      wsUrl = `${proto}://${window.location.host}${apiUrl}/ws`
    }

    try {
      const ws = new WebSocket(wsUrl, [`access_token.${token}`])
      wsRef.current = ws

      ws.onopen = () => {
        retriesRef.current = 0 // reset on successful connection
      }

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
        // Auto-reconnect with max retries
        if (useAuthStore.getState().isAuthenticated && retriesRef.current < WS_MAX_RETRIES) {
          retriesRef.current++
          reconnectTimerRef.current = setTimeout(connect, WS_RECONNECT_DELAY * retriesRef.current)
        }
      }

      ws.onerror = () => {
        ws.close()
      }
    } catch {
      // WebSocket constructor can throw in some environments
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
