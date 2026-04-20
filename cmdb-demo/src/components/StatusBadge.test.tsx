import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import StatusBadge from './StatusBadge'

describe('StatusBadge', () => {
  it('renders the status text verbatim (does not mutate display)', () => {
    render(<StatusBadge status="Operational" />)
    expect(screen.getByText('Operational')).toBeInTheDocument()
  })

  it('normalizes keys to map known statuses regardless of casing', () => {
    // "CRITICAL" should map to the same class bucket as "critical"
    render(<StatusBadge status="CRITICAL" />)
    const badge = screen.getByText('CRITICAL')
    expect(badge.className).toContain('bg-error-container')
    expect(badge.className).toContain('text-on-error-container')
  })

  it('treats spaces/hyphens as underscores when looking up colors', () => {
    // "in progress" → "in_progress" key
    render(<StatusBadge status="in progress" />)
    const badge = screen.getByText('in progress')
    // in_progress uses the primary-container token
    expect(badge.className).toContain('text-on-primary-container')
  })

  it('falls back to a neutral color when status is unknown', () => {
    render(<StatusBadge status="totally-made-up" />)
    const badge = screen.getByText('totally-made-up')
    expect(badge.className).toContain('bg-surface-container-highest')
    expect(badge.className).toContain('text-on-surface-variant')
  })

  it('applies the operational color for the operational status', () => {
    render(<StatusBadge status="operational" />)
    const badge = screen.getByText('operational')
    expect(badge.className).toContain('bg-[#064e3b]')
  })
})
