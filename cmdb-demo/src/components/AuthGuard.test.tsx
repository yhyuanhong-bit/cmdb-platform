import { describe, it, expect, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom'
import AuthGuard from './AuthGuard'
import { useAuthStore } from '../stores/authStore'

// Reset the auth store between tests so state doesn't leak across assertions.
beforeEach(() => {
  useAuthStore.setState({
    accessToken: null,
    refreshToken: null,
    user: null,
    isAuthenticated: false,
  })
})

function LocationProbe() {
  const loc = useLocation()
  return (
    <div>
      <span data-testid="pathname">{loc.pathname}</span>
      <span data-testid="from-pathname">
        {(loc.state as { from?: { pathname: string } } | null)?.from?.pathname ?? ''}
      </span>
    </div>
  )
}

describe('AuthGuard', () => {
  it('redirects to /login when unauthenticated', () => {
    render(
      <MemoryRouter initialEntries={['/assets']}>
        <Routes>
          <Route
            path="/assets"
            element={
              <AuthGuard>
                <div>Protected Assets</div>
              </AuthGuard>
            }
          />
          <Route path="/login" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>,
    )

    expect(screen.queryByText('Protected Assets')).not.toBeInTheDocument()
    expect(screen.getByTestId('pathname').textContent).toBe('/login')
  })

  it('forwards the original pathname on redirect state', () => {
    render(
      <MemoryRouter initialEntries={['/racks']}>
        <Routes>
          <Route
            path="/racks"
            element={
              <AuthGuard>
                <div>Racks</div>
              </AuthGuard>
            }
          />
          <Route path="/login" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>,
    )

    // The originating pathname should round-trip via router state
    expect(screen.getByTestId('from-pathname').textContent).toBe('/racks')
  })

  it('renders children when authenticated', () => {
    useAuthStore.setState({
      accessToken: 'valid-access-token',
      refreshToken: 'valid-refresh-token',
      user: {
        id: 'a0000000-0000-0000-0000-000000000001',
        username: 'testuser',
        display_name: 'Test User',
        email: 'test@example.invalid',
        permissions: {},
      },
      isAuthenticated: true,
    })

    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <Routes>
          <Route
            path="/dashboard"
            element={
              <AuthGuard>
                <div>Protected Dashboard</div>
              </AuthGuard>
            }
          />
          <Route path="/login" element={<div>Login Page</div>} />
        </Routes>
      </MemoryRouter>,
    )

    expect(screen.getByText('Protected Dashboard')).toBeInTheDocument()
    expect(screen.queryByText('Login Page')).not.toBeInTheDocument()
  })
})
