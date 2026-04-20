import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import Login from './Login'
import { useAuthStore } from '../stores/authStore'

// Reset auth store + fetch between tests
beforeEach(() => {
  useAuthStore.setState({
    accessToken: null,
    refreshToken: null,
    user: null,
    isAuthenticated: false,
  })
})

afterEach(() => {
  vi.restoreAllMocks()
})

function renderLogin() {
  return render(
    <MemoryRouter initialEntries={['/login']}>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/locations" element={<div>Locations Page</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('Login', () => {
  it('renders username and password fields plus a submit button', () => {
    renderLogin()

    expect(screen.getByPlaceholderText('Enter username')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Enter password')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /login/i })).toBeInTheDocument()
  })

  it('marks username and password as required (native form validation)', () => {
    renderLogin()

    const username = screen.getByPlaceholderText('Enter username') as HTMLInputElement
    const password = screen.getByPlaceholderText('Enter password') as HTMLInputElement

    expect(username.required).toBe(true)
    expect(password.required).toBe(true)
    expect(password.type).toBe('password')
  })

  it('navigates to /locations on successful login', async () => {
    const user = userEvent.setup()
    globalThis.fetch = vi
      .fn()
      // First call: login endpoint
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          data: {
            access_token: 'fake-access-token',
            refresh_token: 'fake-refresh-token',
            expires_in: 3600,
          },
        }),
      })
      // Second call: fetch current user
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          data: {
            id: 'a0000000-0000-0000-0000-000000000001',
            username: 'testuser',
            display_name: 'Test User',
            email: 'test@example.invalid',
            permissions: {},
          },
        }),
      }) as unknown as typeof fetch

    renderLogin()

    await user.type(screen.getByPlaceholderText('Enter username'), 'testuser')
    await user.type(screen.getByPlaceholderText('Enter password'), 'correct-horse')
    await user.click(screen.getByRole('button', { name: /login/i }))

    await waitFor(() => {
      expect(screen.getByText('Locations Page')).toBeInTheDocument()
    })
  })

  it('shows an error banner on failed login', async () => {
    const user = userEvent.setup()
    globalThis.fetch = vi.fn().mockResolvedValueOnce({
      ok: false,
      json: () => Promise.resolve({ error: 'invalid credentials' }),
    }) as unknown as typeof fetch

    renderLogin()

    await user.type(screen.getByPlaceholderText('Enter username'), 'testuser')
    await user.type(screen.getByPlaceholderText('Enter password'), 'wrong-password')
    await user.click(screen.getByRole('button', { name: /login/i }))

    await waitFor(() => {
      expect(screen.getByText(/login failed/i)).toBeInTheDocument()
    })
    // Still on login page
    expect(screen.queryByText('Locations Page')).not.toBeInTheDocument()
  })

  it('disables the submit button while logging in', async () => {
    const user = userEvent.setup()
    // Never resolve so we can observe the pending state
    let resolveLogin: (v: unknown) => void = () => {}
    const loginPromise = new Promise((resolve) => {
      resolveLogin = resolve
    })
    globalThis.fetch = vi.fn().mockReturnValueOnce(loginPromise) as unknown as typeof fetch

    renderLogin()

    await user.type(screen.getByPlaceholderText('Enter username'), 'testuser')
    await user.type(screen.getByPlaceholderText('Enter password'), 'password')

    const btn = screen.getByRole('button', { name: /login/i })
    await user.click(btn)

    // Button should now be disabled and show "Logging in..." copy
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /logging in/i })).toBeDisabled()
    })

    // Clean up: resolve the promise so we don't leak
    resolveLogin({
      ok: false,
      json: () => Promise.resolve({ error: 'aborted' }),
    })
  })
})
