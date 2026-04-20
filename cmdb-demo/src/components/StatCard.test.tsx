import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import StatCard from './StatCard'

describe('StatCard', () => {
  it('renders label, value, and icon', () => {
    // Arrange + Act
    render(<StatCard icon="bolt" label="Operational Assets" value={42} />)

    // Assert
    expect(screen.getByText('Operational Assets')).toBeInTheDocument()
    expect(screen.getByText('42')).toBeInTheDocument()
    // material-symbols span renders the icon name as text
    expect(screen.getByText('bolt')).toBeInTheDocument()
  })

  it('renders string values verbatim', () => {
    render(<StatCard icon="memory" label="CPU" value="12%" />)
    expect(screen.getByText('12%')).toBeInTheDocument()
  })

  it('omits the sub line when no sub prop is given', () => {
    render(<StatCard icon="bolt" label="Uptime" value="99.9%" />)
    // Sub is the only element that could hold extra copy — when absent,
    // only label, value, and icon should be present.
    expect(screen.queryByText(/last 24h/i)).not.toBeInTheDocument()
  })

  it('renders sub when provided', () => {
    render(<StatCard icon="bolt" label="Uptime" value="99.9%" sub="last 24h" />)
    expect(screen.getByText('last 24h')).toBeInTheDocument()
  })

  it('applies custom subColor class when provided', () => {
    render(
      <StatCard
        icon="bolt"
        label="Uptime"
        value="99.9%"
        sub="down 0.1%"
        subColor="text-error"
      />,
    )
    const sub = screen.getByText('down 0.1%')
    expect(sub.className).toContain('text-error')
  })
})
