import { ReactNode } from 'react'
import { useWebSocket } from '../hooks/useWebSocket'

export default function WebSocketProvider({ children }: { children: ReactNode }) {
  useWebSocket()
  return <>{children}</>
}
