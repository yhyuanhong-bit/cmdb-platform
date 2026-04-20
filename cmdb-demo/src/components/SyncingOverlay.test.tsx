import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import SyncingOverlay from './SyncingOverlay'

describe('SyncingOverlay', () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('renders nothing initially (idle)', () => {
    const { container } = render(<SyncingOverlay />)
    expect(container.firstChild).toBeNull()
  })

  it('shows the overlay when the sync-in-progress event fires', () => {
    render(<SyncingOverlay />)

    act(() => {
      window.dispatchEvent(new Event('sync-in-progress'))
    })

    expect(screen.getByText('Initial Sync in Progress')).toBeInTheDocument()
    expect(screen.getByText(/Checking every 5 seconds/i)).toBeInTheDocument()
  })

  it('polls /readyz on an interval while visible', () => {
    vi.useFakeTimers()
    const fetchMock = vi
      .fn()
      .mockResolvedValue({ ok: false }) // not ready yet
    globalThis.fetch = fetchMock as unknown as typeof fetch

    render(<SyncingOverlay />)

    act(() => {
      window.dispatchEvent(new Event('sync-in-progress'))
    })

    // Nothing polled yet — interval fires after 5s
    expect(fetchMock).not.toHaveBeenCalled()

    act(() => {
      vi.advanceTimersByTime(5000)
    })

    expect(fetchMock).toHaveBeenCalledWith('/readyz')
  })

  it('stays visible on fetch failure (network error)', async () => {
    vi.useFakeTimers()
    globalThis.fetch = vi
      .fn()
      .mockRejectedValue(new Error('network down')) as unknown as typeof fetch

    render(<SyncingOverlay />)

    act(() => {
      window.dispatchEvent(new Event('sync-in-progress'))
    })

    // Let a tick pass so the async rejection resolves, then verify overlay stays.
    await act(async () => {
      vi.advanceTimersByTime(5000)
      await Promise.resolve()
    })

    expect(screen.getByText('Initial Sync in Progress')).toBeInTheDocument()
  })
})
