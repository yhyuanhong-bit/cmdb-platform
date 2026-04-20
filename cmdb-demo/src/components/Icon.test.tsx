import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import Icon from './Icon'

describe('Icon', () => {
  it('renders the icon name as its text content', () => {
    render(<Icon name="dashboard" />)
    expect(screen.getByText('dashboard')).toBeInTheDocument()
  })

  it('always applies the material-symbols-outlined base class', () => {
    const { container } = render(<Icon name="settings" />)
    const span = container.firstChild as HTMLElement
    expect(span.className).toContain('material-symbols-outlined')
  })

  it('appends custom className when provided', () => {
    const { container } = render(
      <Icon name="warning" className="text-error text-lg" />,
    )
    const span = container.firstChild as HTMLElement
    expect(span.className).toContain('material-symbols-outlined')
    expect(span.className).toContain('text-error')
    expect(span.className).toContain('text-lg')
  })
})
