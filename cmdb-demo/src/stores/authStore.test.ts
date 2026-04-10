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

    const success = await useAuthStore.getState().login('admin', 'pass')
    expect(success).toBe(true)

    const state = useAuthStore.getState()
    expect(state.isAuthenticated).toBe(true)
    expect(state.accessToken).toBe('test-access')
    expect(state.refreshToken).toBe('test-refresh')
    expect(state.user?.username).toBe('admin')
  })

  it('login returns false on failure', async () => {
    globalThis.fetch = vi.fn().mockResolvedValueOnce({
      ok: false,
      json: () => Promise.resolve({ error: 'invalid credentials' }),
    })

    const success = await useAuthStore.getState().login('bad', 'bad')
    expect(success).toBe(false)
    expect(useAuthStore.getState().isAuthenticated).toBe(false)
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
