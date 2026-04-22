import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'

// Mock every hook Dashboard reads. vi.mock is hoisted, so the factory runs
// before the Dashboard module evaluates.
vi.mock('../hooks/useDashboard', () => ({
  useDashboardStats: vi.fn(),
  useRackStats: vi.fn(),
  useLifecycleStats: vi.fn(),
}))
vi.mock('../hooks/useMonitoring', () => ({
  useAlerts: vi.fn(),
}))
vi.mock('../hooks/useBIA', () => ({
  useBIAStats: vi.fn(),
}))
vi.mock('../hooks/useInventory', () => ({
  useInventoryTasks: vi.fn(),
  useTaskSummary: vi.fn(),
}))
vi.mock('../contexts/LocationContext', () => ({
  useLocationContext: () => ({
    path: {},
    currentLevel: 'global',
    setPath: vi.fn(),
    navigateToLevel: vi.fn(),
    breadcrumbs: [],
  }),
}))
vi.mock('../components/LocationBreadcrumb', () => ({
  default: () => null,
}))

import Dashboard from './Dashboard'
import { useDashboardStats, useRackStats, useLifecycleStats } from '../hooks/useDashboard'
import { useAlerts } from '../hooks/useMonitoring'
import { useBIAStats } from '../hooks/useBIA'
import { useInventoryTasks, useTaskSummary } from '../hooks/useInventory'

const mockedDashboardStats = vi.mocked(useDashboardStats)
const mockedRackStats = vi.mocked(useRackStats)
const mockedLifecycleStats = vi.mocked(useLifecycleStats)
const mockedAlerts = vi.mocked(useAlerts)
const mockedBIAStats = vi.mocked(useBIAStats)
const mockedInventoryTasks = vi.mocked(useInventoryTasks)
const mockedTaskSummary = vi.mocked(useTaskSummary)

function stubQuery<R>(overrides: Partial<{ data: unknown; isLoading: boolean; error: unknown; refetch: () => void }> = {}): R {
  return {
    data: undefined,
    isLoading: false,
    error: null,
    refetch: vi.fn(),
    ...overrides,
  } as unknown as R
}

function renderDashboard() {
  return render(
    <MemoryRouter>
      <Dashboard />
    </MemoryRouter>,
  )
}

beforeEach(() => {
  mockedRackStats.mockReturnValue(stubQuery<ReturnType<typeof useRackStats>>({ data: { occupancy_pct: 0, total_u: 0, used_u: 0 } }))
  mockedLifecycleStats.mockReturnValue(stubQuery<ReturnType<typeof useLifecycleStats>>({ data: { data: { by_status: {} } } }))
  mockedAlerts.mockReturnValue(stubQuery<ReturnType<typeof useAlerts>>({ data: { data: [] } }))
  mockedBIAStats.mockReturnValue(stubQuery<ReturnType<typeof useBIAStats>>({ data: { data: { by_tier: {} } } }))
  mockedInventoryTasks.mockReturnValue(stubQuery<ReturnType<typeof useInventoryTasks>>({ data: { data: [] } }))
  mockedTaskSummary.mockReturnValue(stubQuery<ReturnType<typeof useTaskSummary>>({ data: { data: { completion_pct: 0, scanned: 0, total: 0 } } }))
})

describe('Dashboard', () => {
  it('shows a loading spinner while stats are loading', () => {
    mockedDashboardStats.mockReturnValue(stubQuery<ReturnType<typeof useDashboardStats>>({ isLoading: true }))

    const { container } = renderDashboard()

    // Spinner = <div class="animate-spin ..." />
    expect(container.querySelector('.animate-spin')).toBeInTheDocument()
  })

  it('renders stat cards with data after stats load', () => {
    mockedDashboardStats.mockReturnValue(stubQuery<ReturnType<typeof useDashboardStats>>({
      data: {
        data: {
          total_assets: 1234,
          total_racks: 42,
          critical_alerts: 3,
          active_orders: 7,
          pending_work_orders: 5,
          energy_current_kw: 12.3,
          rack_utilization_pct: 68,
          avg_quality_score: 0.92,
        },
      },
    }))
    mockedRackStats.mockReturnValue(stubQuery<ReturnType<typeof useRackStats>>({ data: { occupancy_pct: 68, total_u: 100, used_u: 68 } }))

    renderDashboard()

    expect(screen.getByText('1,234')).toBeInTheDocument()
    // Critical alerts render zero-padded to two digits
    expect(screen.getByText('03')).toBeInTheDocument()
    // Rack occupancy is shown as a percentage
    expect(screen.getByText('68%')).toBeInTheDocument()
  })

  it('renders a retry affordance when stats fail to load', () => {
    const refetch = vi.fn()
    mockedDashboardStats.mockReturnValue(stubQuery<ReturnType<typeof useDashboardStats>>({
      error: new Error('boom'),
      refetch,
    }))

    renderDashboard()

    expect(screen.getByText(/failed to load dashboard stats/i)).toBeInTheDocument()
    const retry = screen.getByRole('button', { name: /retry/i })
    retry.click()
    expect(refetch).toHaveBeenCalledTimes(1)
  })
})
