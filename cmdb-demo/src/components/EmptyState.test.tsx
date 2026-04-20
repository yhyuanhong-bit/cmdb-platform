import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import EmptyState from './EmptyState'

describe('EmptyState', () => {
  it('renders the title as a heading', () => {
    render(<EmptyState title="No assets yet" />)
    const heading = screen.getByRole('heading', { name: 'No assets yet' })
    expect(heading).toBeInTheDocument()
  })

  it('renders the description when provided', () => {
    render(
      <EmptyState
        title="Nothing here"
        description="Create your first asset to get started."
      />,
    )
    expect(
      screen.getByText('Create your first asset to get started.'),
    ).toBeInTheDocument()
  })

  it('omits the description block when none is provided', () => {
    render(<EmptyState title="Empty" />)
    // Description uses <p> — make sure no paragraph is rendered.
    expect(screen.queryByText(/./, { selector: 'p' })).not.toBeInTheDocument()
  })

  it('renders the optional action node', () => {
    render(
      <EmptyState
        title="No results"
        action={<button type="button">Retry</button>}
      />,
    )
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
  })

  it('uses the warning tone classes when tone is "warning"', () => {
    const { container } = render(
      <EmptyState title="Caution" tone="warning" />,
    )
    // The icon badge wrapper is the first div child with the tinted ring bg.
    const badge = container.querySelector('[aria-hidden="true"]')
    expect(badge).not.toBeNull()
    expect(badge?.className).toContain('ring-[#ffa94d]/25')
  })

  it('uses compact spacing when compact is true', () => {
    const { container } = render(
      <EmptyState title="Compact" compact />,
    )
    // Outer wrapper is the only top-level flex column with py-6 in compact
    const wrapper = container.firstChild as HTMLElement
    expect(wrapper.className).toContain('py-6')
    expect(wrapper.className).not.toContain('py-10')
  })
})
