import { create } from 'zustand'
import { persist, createJSONStorage } from 'zustand/middleware'
import type { CurrentUser, TokenPair } from '../lib/api/types'

const API_URL = import.meta.env.VITE_API_URL || '/api/v1'

interface AuthState {
  accessToken: string | null
  refreshToken: string | null
  user: CurrentUser | null
  isAuthenticated: boolean

  login: (username: string, password: string, tenantSlug?: string) => Promise<LoginResult>
  logout: () => void
  refreshTokens: () => Promise<boolean>
  fetchCurrentUser: () => Promise<void>
}

// LoginResult is what the login form needs to render the right error
// message. We deliberately distinguish "wrong credentials" from
// "rate-limited / server-down" so users do not get told to recheck
// their password when the real problem is the rate limiter (5/min/IP)
// or a backend outage.
export type LoginResult =
  | { ok: true }
  | { ok: false; reason: 'invalid_credentials' | 'rate_limited' | 'server_unavailable' | 'network'; retryAfterSeconds?: number }

const DEFAULT_TENANT_SLUG = import.meta.env.VITE_DEFAULT_TENANT_SLUG || 'tw'

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
  accessToken: null,
  refreshToken: null,
  user: null,
  isAuthenticated: false,

  login: async (username, password, tenantSlug): Promise<LoginResult> => {
    try {
      const slug = tenantSlug?.trim() || DEFAULT_TENANT_SLUG
      const res = await fetch(`${API_URL}/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ tenant_slug: slug, username, password }),
      })
      // Some error responses may not be JSON (e.g. proxy 502). Guard
      // the parse so a non-JSON 5xx still produces a useful result.
      const json = await res.json().catch(() => ({}))
      if (!res.ok) {
        if (import.meta.env.DEV) {
          console.error('Login failed:', res.status, json)
        }
        if (res.status === 429) {
          const retryAfter = parseInt(res.headers.get('retry-after') || '60', 10)
          return { ok: false, reason: 'rate_limited', retryAfterSeconds: Number.isFinite(retryAfter) ? retryAfter : 60 }
        }
        if (res.status >= 500) {
          return { ok: false, reason: 'server_unavailable' }
        }
        // 401 / 403 / 400 → treat as bad credentials.
        return { ok: false, reason: 'invalid_credentials' }
      }

      const tokens = json.data as TokenPair
      set({
        accessToken: tokens.access_token,
        refreshToken: tokens.refresh_token,
        isAuthenticated: true,
      })

      await get().fetchCurrentUser()
      return { ok: true }
    } catch (err) {
      if (import.meta.env.DEV) {
        console.error('Login network error (CORS or server unreachable?):', err)
      }
      return { ok: false, reason: 'network' }
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
