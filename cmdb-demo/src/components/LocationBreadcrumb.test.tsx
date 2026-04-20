import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom'
import type { ReactNode } from 'react'
import LocationBreadcrumb from './LocationBreadcrumb'
import { LocationProvider, useLocationContext } from '../contexts/LocationContext'
import type { LocationPath } from '../contexts/LocationContext'

// Helper: hydrates the LocationContext with a given path before rendering children.
// Using a component rather than a custom hook so it can be composed in JSX.
function PathSeeder({ path, children }: { path: LocationPath; children: ReactNode }) {
  const ctx = useLocationContext()
  // Seed once (StrictMode renders twice — idempotent setter is fine).
  if (JSON.stringify(ctx.path) !== JSON.stringify(path)) {
    ctx.setPath(path)
  }
  return <>{children}</>
}

function renderWithPath(path: LocationPath, suffix?: ReactNode) {
  return render(
    <MemoryRouter initialEntries={['/']}>
      <LocationProvider>
        <PathSeeder path={path}>
          <Routes>
            <Route
              path="/"
              element={<LocationBreadcrumb suffix={suffix} />}
            />
            <Route
              path="/locations"
              element={<LocationMarker>at-locations</LocationMarker>}
            />
            <Route
              path="/locations/*"
              element={<LocationMarker>at-nested</LocationMarker>}
            />
          </Routes>
        </PathSeeder>
      </LocationProvider>
    </MemoryRouter>,
  )
}

function LocationMarker({ children }: { children: ReactNode }) {
  const loc = useLocation()
  return <div data-testid="route-marker">{String(children)}:{loc.pathname}</div>
}

describe('LocationBreadcrumb', () => {
  it('shows only "Global" when the path is empty', () => {
    renderWithPath({})
    expect(screen.getByText('Global')).toBeInTheDocument()
    // No separator character should be present when there's only one crumb
    expect(screen.queryByText('›')).not.toBeInTheDocument()
  })

  it('renders crumbs for each filled location level', () => {
    renderWithPath({
      territory: { id: 'a0000000-0000-0000-0000-000000000001', slug: 'tw', name: 'Taiwan', nameEn: 'Taiwan' },
      region: { id: 'a0000000-0000-0000-0000-000000000002', slug: 'north', name: '北區', nameEn: 'North' },
    })

    expect(screen.getByText('Global')).toBeInTheDocument()
    expect(screen.getByText('Taiwan')).toBeInTheDocument()
    // Fallback language is zh-TW — label (not labelEn) is used
    expect(screen.getByText('北區')).toBeInTheDocument()
  })

  it('navigates to /locations when the Global crumb is clicked', async () => {
    const user = userEvent.setup()
    renderWithPath({
      territory: { id: 'a0000000-0000-0000-0000-000000000001', slug: 'tw', name: 'Taiwan', nameEn: 'Taiwan' },
    })

    // "Global" is clickable when there is at least one child crumb
    await user.click(screen.getByText('Global'))
    expect(screen.getByTestId('route-marker').textContent).toContain('at-locations')
  })

  it('marks the last crumb as non-interactive (bold/semibold, no role=link)', () => {
    renderWithPath({
      territory: { id: 'a0000000-0000-0000-0000-000000000001', slug: 'tw', name: 'Taiwan', nameEn: 'Taiwan' },
    })
    // Only one descendant crumb — "Taiwan" is the last, rendered as a plain span.
    const last = screen.getByText('Taiwan')
    expect(last.tagName).toBe('SPAN')
    expect(last.className).toContain('font-semibold')
  })

  it('appends a suffix when provided', () => {
    renderWithPath(
      {
        territory: { id: 'a0000000-0000-0000-0000-000000000001', slug: 'tw', name: 'Taiwan', nameEn: 'Taiwan' },
      },
      'Asset Details',
    )
    expect(screen.getByText('Asset Details')).toBeInTheDocument()
  })
})
