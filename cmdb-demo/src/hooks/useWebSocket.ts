import { useEffect, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useAuthStore } from '../stores/authStore'

const WS_RECONNECT_DELAY = 3000
const WS_MAX_RETRIES = 5

export function useWebSocket() {
  const token = useAuthStore((s) => s.accessToken)
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const queryClient = useQueryClient()
  const wsRef = useRef<WebSocket | null>(null)

  const invalidateByEvent = (type: string) => {
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
  }

  const invalidateByEventRef = useRef(invalidateByEvent)
  invalidateByEventRef.current = invalidateByEvent

  useEffect(() => {
    if (!token || !isAuthenticated) return

    // Derive WS URL
    const wsBase = import.meta.env.VITE_WS_URL
    const apiUrl = import.meta.env.VITE_API_URL || '/api/v1'
    let wsUrl: string
    if (wsBase) {
      // Explicit WS URL (production)
      wsUrl = wsBase
    } else if (apiUrl.startsWith('http')) {
      // Absolute API URL: replace protocol
      const wsProtocol = apiUrl.startsWith('https') ? 'wss' : 'ws'
      wsUrl = apiUrl.replace(/^https?/, wsProtocol) + '/ws'
    } else {
      // Relative API URL: use same host:port (goes through Vite proxy in dev)
      const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
      wsUrl = `${proto}://${window.location.host}${apiUrl}/ws`
    }

    let retries = 0
    let timer: ReturnType<typeof setTimeout> | undefined

    async function doConnect() {
      // Use the latest token (may have been refreshed)
      const currentToken = useAuthStore.getState().accessToken
      if (!currentToken) return

      try {
        const ws = new WebSocket(wsUrl, [`access_token.${currentToken}`])
        wsRef.current = ws

        ws.onopen = () => {
          retries = 0
        }

        ws.onmessage = (event) => {
          try {
            const msg = JSON.parse(event.data)
            if (msg.type) {
              invalidateByEventRef.current(msg.type)
            }
          } catch {
            // ignore malformed messages
          }
        }

        ws.onclose = async () => {
          wsRef.current = null
          if (!useAuthStore.getState().isAuthenticated) return

          // Try refreshing the token before reconnecting
          if (retries === 0 || retries === WS_MAX_RETRIES) {
            await useAuthStore.getState().refreshTokens()
          }

          if (retries < WS_MAX_RETRIES) {
            retries++
            timer = setTimeout(doConnect, WS_RECONNECT_DELAY * retries)
          } else {
            // All fast retries exhausted — slow poll every 30s (single attempt each time)
            timer = setTimeout(doConnect, 30_000)
          }
        }

        ws.onerror = () => {
          ws.close()
        }
      } catch {
        // WebSocket constructor can throw in some environments
      }
    }

    doConnect()

    return () => {
      if (timer) clearTimeout(timer)
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
    }
  }, [token, isAuthenticated])
}
