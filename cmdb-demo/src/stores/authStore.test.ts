import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { useAuthStore } from './authStore'

// Reset store state between tests
beforeEach(() => {
  useAuthStore.setState({
    accessToken: null,
    refreshToken: null,
    user: null,
    isAuthenticated: false,
  })
  sessionStorage.clear()
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('authStore', () => {
  it('starts unauthenticated', () => {
    const state = useAuthStore.getState()
    expect(state.isAuthenticated).toBe(false)
    expect(state.accessToken).toBeNull()
    expect(state.user).toBeNull()
  })

  it('login sets tokens and isAuthenticated on success', async () => {
    // Mock successful login response then fetchCurrentUser
    globalThis.fetch = vi.fn()
      .mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              access_token: 'test-access',
              refresh_token: 'test-refresh',
              expires_in: 3600,
            },
          }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              id: '1',
              username: 'admin',
              display_name: 'Admin',
              email: 'a@b.com',
              permissions: {},
            },
          }),
      })

    const result = await useAuthStore.getState().login('admin', 'pass')
    expect(result.ok).toBe(true)

    const state = useAuthStore.getState()
    expect(state.isAuthenticated).toBe(true)
    expect(state.accessToken).toBe('test-access')
    expect(state.refreshToken).toBe('test-refresh')
    expect(state.user?.username).toBe('admin')
  })

  it('login returns invalid_credentials on 401', async () => {
    globalThis.fetch = vi.fn().mockResolvedValueOnce({
      ok: false,
      status: 401,
      headers: { get: () => null },
      json: () => Promise.resolve({ error: 'invalid credentials' }),
    })

    const result = await useAuthStore.getState().login('bad', 'bad')
    expect(result).toEqual({ ok: false, reason: 'invalid_credentials' })
    expect(useAuthStore.getState().isAuthenticated).toBe(false)
  })

  it('login returns rate_limited on 429 with retry-after', async () => {
    globalThis.fetch = vi.fn().mockResolvedValueOnce({
      ok: false,
      status: 429,
      headers: { get: (h: string) => (h.toLowerCase() === 'retry-after' ? '45' : null) },
      json: () => Promise.resolve({ error: { code: 'RATE_LIMITED' } }),
    })

    const result = await useAuthStore.getState().login('admin', 'pass')
    expect(result).toEqual({ ok: false, reason: 'rate_limited', retryAfterSeconds: 45 })
  })

  it('login returns server_unavailable on 5xx', async () => {
    globalThis.fetch = vi.fn().mockResolvedValueOnce({
      ok: false,
      status: 503,
      headers: { get: () => null },
      json: () => Promise.resolve({}),
    })

    const result = await useAuthStore.getState().login('admin', 'pass')
    expect(result).toEqual({ ok: false, reason: 'server_unavailable' })
  })

  it('login returns network on fetch throw', async () => {
    globalThis.fetch = vi.fn().mockRejectedValueOnce(new TypeError('failed to fetch'))

    const result = await useAuthStore.getState().login('admin', 'pass')
    expect(result).toEqual({ ok: false, reason: 'network' })
  })

  it('logout clears all state', () => {
    // Set up authenticated state
    useAuthStore.setState({
      accessToken: 'token',
      refreshToken: 'refresh',
      user: {
        id: '1',
        username: 'admin',
        display_name: 'Admin',
        email: '',
        permissions: {},
      },
      isAuthenticated: true,
    })

    useAuthStore.getState().logout()

    const state = useAuthStore.getState()
    expect(state.isAuthenticated).toBe(false)
    expect(state.accessToken).toBeNull()
    expect(state.refreshToken).toBeNull()
    expect(state.user).toBeNull()
  })

  it('refreshTokens rotates tokens on success', async () => {
    useAuthStore.setState({
      accessToken: 'old-access',
      refreshToken: 'old-refresh',
      isAuthenticated: true,
    })

    globalThis.fetch = vi.fn().mockResolvedValueOnce({
      ok: true,
      json: () =>
        Promise.resolve({
          data: {
            access_token: 'new-access',
            refresh_token: 'new-refresh',
            expires_in: 3600,
          },
        }),
    })

    const success = await useAuthStore.getState().refreshTokens()
    expect(success).toBe(true)
    expect(useAuthStore.getState().accessToken).toBe('new-access')
    expect(useAuthStore.getState().refreshToken).toBe('new-refresh')
  })

  it('refreshTokens returns false when no refresh token', async () => {
    const success = await useAuthStore.getState().refreshTokens()
    expect(success).toBe(false)
  })
})
