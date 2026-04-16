import { useEffect, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useAuthStore } from '../stores/authStore'

const WS_RECONNECT_DELAY = 3000
const WS_MAX_RETRIES = 3

export function useWebSocket() {
  const token = useAuthStore((s) => s.accessToken)
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const queryClient = useQueryClient()
  const wsRef = useRef<WebSocket | null>(null)
  const retriesRef = useRef(0)
  const gaveUpRef = useRef(false)

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
    // If we already gave up in a previous mount cycle, don't retry
    if (gaveUpRef.current) return

    // Derive WS URL
    const wsBase = import.meta.env.VITE_WS_URL
    const apiUrl = import.meta.env.VITE_API_URL || '/api/v1'
    let wsUrl: string
    if (wsBase) {
      wsUrl = wsBase
    } else if (apiUrl.startsWith('http')) {
      const wsProtocol = apiUrl.startsWith('https') ? 'wss' : 'ws'
      wsUrl = apiUrl.replace(/^https?/, wsProtocol) + '/ws'
    } else {
      const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
      wsUrl = `${proto}://${window.location.host}${apiUrl}/ws`
    }

    let timer: ReturnType<typeof setTimeout> | undefined
    let stopped = false

    function doConnect() {
      if (stopped || gaveUpRef.current) return

      const currentToken = useAuthStore.getState().accessToken
      if (!currentToken) return

      // Close existing connection if any (Strict Mode double-invoke protection)
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }

      try {
        const ws = new WebSocket(wsUrl, [`access_token.${currentToken}`])
        wsRef.current = ws

        ws.onopen = () => {
          retriesRef.current = 0
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

        ws.onclose = () => {
          wsRef.current = null
          if (stopped || gaveUpRef.current || !useAuthStore.getState().isAuthenticated) return

          retriesRef.current++
          if (retriesRef.current > WS_MAX_RETRIES) {
            gaveUpRef.current = true
            return
          }
          timer = setTimeout(doConnect, WS_RECONNECT_DELAY * retriesRef.current)
        }

        ws.onerror = () => {
          // Don't log — the browser already logs WebSocket errors.
          // Just close to trigger onclose → retry logic.
          ws.close()
        }
      } catch {
        // WebSocket constructor can throw (e.g., invalid URL)
        gaveUpRef.current = true
      }
    }

    doConnect()

    return () => {
      stopped = true
      if (timer) clearTimeout(timer)
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
    }
  }, [token, isAuthenticated])
}
