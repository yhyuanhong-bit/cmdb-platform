import { create } from 'zustand'
import { persist, createJSONStorage } from 'zustand/middleware'
import type { CurrentUser, TokenPair } from '../lib/api/types'

const API_URL = import.meta.env.VITE_API_URL || '/api/v1'

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

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
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
        if (import.meta.env.DEV) {
          console.error('Login failed:', json)
        }
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
      if (import.meta.env.DEV) {
        console.error('Login network error (CORS or server unreachable?):', err)
      }
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
    }),
    {
      name: 'cmdb-auth',
      storage: createJSONStorage(() => sessionStorage),
      partialize: (state) => ({
        accessToken: state.accessToken,
        refreshToken: state.refreshToken,
        user: state.user,
        isAuthenticated: state.isAuthenticated,
      }),
    }
  )
)
