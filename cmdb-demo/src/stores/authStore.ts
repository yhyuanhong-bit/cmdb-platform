import { create } from 'zustand'
import type { CurrentUser, TokenPair } from '../lib/api/types'

const API_URL = import.meta.env.VITE_API_URL || 'http://10.134.143.218:8080/api/v1'

interface AuthState {
  accessToken: string | null
  refreshToken: string | null
  user: CurrentUser | null
  isAuthenticated: boolean

  login: (username: string, password: string) => Promise<boolean>
  logout: () => void
  refreshTokens: () => Promise<boolean>
  fetchCurrentUser: () => Promise<void>
}

export const useAuthStore = create<AuthState>((set, get) => ({
  accessToken: null,
  refreshToken: null,
  user: null,
  isAuthenticated: false,

  login: async (username, password) => {
    try {
      const res = await fetch(`${API_URL}/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      })
      const json = await res.json()
      if (!res.ok) {
        console.error('Login failed:', json)
        return false
      }

      const tokens = json.data as TokenPair
      set({
        accessToken: tokens.access_token,
        refreshToken: tokens.refresh_token,
        isAuthenticated: true,
      })

      await get().fetchCurrentUser()
      return true
    } catch (err) {
      console.error('Login network error (CORS or server unreachable?):', err)
      return false
    }
  },

  logout: () => {
    set({
      accessToken: null,
      refreshToken: null,
      user: null,
      isAuthenticated: false,
    })
  },

  refreshTokens: async () => {
    const { refreshToken } = get()
    if (!refreshToken) return false

    try {
      const res = await fetch(`${API_URL}/auth/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: refreshToken }),
      })
      const json = await res.json()
      if (!res.ok) return false

      const tokens = json.data as TokenPair
      set({
        accessToken: tokens.access_token,
        refreshToken: tokens.refresh_token,
      })
      return true
    } catch {
      return false
    }
  },

  fetchCurrentUser: async () => {
    const { accessToken } = get()
    if (!accessToken) return

    try {
      const res = await fetch(`${API_URL}/auth/me`, {
        headers: { Authorization: `Bearer ${accessToken}` },
      })
      const json = await res.json()
      if (res.ok) {
        set({ user: json.data as CurrentUser })
      }
    } catch {
      // ignore
    }
  },
}))
