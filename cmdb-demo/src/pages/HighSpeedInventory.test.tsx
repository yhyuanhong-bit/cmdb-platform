import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactElement } from 'react'

vi.mock('../contexts/LocationContext', () => ({
  useLocationContext: () => ({
    path: {},
    currentLevel: 'global',
    setPath: vi.fn(),
    navigateToLevel: vi.fn(),
    breadcrumbs: [],
  }),
}))

vi.mock('../hooks/useInventory', () => ({
  useInventoryTasks: vi.fn(),
  useCompleteTask: () => ({ mutate: vi.fn(), isPending: false }),
  useImportInventoryItems: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteInventoryTask: () => ({ mutate: vi.fn(), isPending: false }),
  useResolveDiscrepancy: () => ({ mutate: vi.fn(), isPending: false }),
}))

// Stub the raw apiClient so the component's inline useQuery calls resolve
// to empty data without hitting the network.
vi.mock('../lib/api/client', () => ({
  apiClient: {
    get: vi.fn().mockResolvedValue({}),
    post: vi.fn().mockResolvedValue({}),
    put: vi.fn().mockResolvedValue({}),
    patch: vi.fn().mockResolvedValue({}),
    del: vi.fn().mockResolvedValue(undefined),
  },
}))

// The QR scanner and create-task modal pull in heavy deps (video, xlsx) that
// aren't relevant for page-level render assertions. Stub them out.
vi.mock('../components/CreateInventoryTaskModal', () => ({
  default: () => null,
}))
vi.mock('../components/QRScanner', () => ({
  default: () => null,
}))

import HighSpeedInventory from './HighSpeedInventory'
import { useInventoryTasks } from '../hooks/useInventory'

const mockedInventoryTasks = vi.mocked(useInventoryTasks)

function stubQuery<R>(overrides: Partial<{ data: unknown; isLoading: boolean; error: unknown }> = {}): R {
  return {
    data: undefined,
    isLoading: false,
    error: null,
    refetch: vi.fn(),
    ...overrides,
  } as unknown as R
}

function renderInventory(ui: ReactElement = <HighSpeedInventory />) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  mockedInventoryTasks.mockReset()
})

describe('HighSpeedInventory', () => {
  it('renders the loading spinner while tasks are loading', () => {
    mockedInventoryTasks.mockReturnValue(stubQuery<ReturnType<typeof useInventoryTasks>>({ isLoading: true }))

    const { container } = renderInventory()

    expect(container.querySelector('.animate-spin')).toBeInTheDocument()
    // Title hasn't rendered yet
    expect(screen.queryByRole('heading', { level: 1 })).not.toBeInTheDocument()
  })

  it('renders the page title and empty-state placeholders when no active task exists', () => {
    mockedInventoryTasks.mockReturnValue(stubQuery<ReturnType<typeof useInventoryTasks>>({ data: { data: [] } }))

    renderInventory()

    // Inventory title always shows (even with no active task)
    expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument()
    // With no task, task_id/operator/started fall back to em-dash
    expect(screen.getAllByText('—').length).toBeGreaterThanOrEqual(2)
  })

  it('renders task metadata when an in-progress task is returned', () => {
    mockedInventoryTasks.mockReturnValue(stubQuery<ReturnType<typeof useInventoryTasks>>({
      data: {
        data: [
          {
            id: 'task-123',
            code: 'INV-2026-042',
            name: 'Q2 IDC-01 Audit',
            status: 'in_progress',
            assigned_to: 'ops-bot',
            planned_date: '2026-04-22',
          },
        ],
      },
    }))

    renderInventory()

    expect(screen.getByText('INV-2026-042')).toBeInTheDocument()
    expect(screen.getByText('ops-bot')).toBeInTheDocument()
    expect(screen.getByText('2026-04-22')).toBeInTheDocument()
    // Complete-task button shows for in_progress tasks
    expect(screen.getByRole('button', { name: /complete task/i })).toBeInTheDocument()
  })
})
