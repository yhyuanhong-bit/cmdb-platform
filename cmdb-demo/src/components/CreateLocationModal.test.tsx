import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import CreateLocationModal from './CreateLocationModal'

describe('CreateLocationModal', () => {
  it('renders nothing when closed', () => {
    const { container } = render(
      <CreateLocationModal open={false} onClose={() => {}} />,
    )
    expect(container.firstChild).toBeNull()
  })

  it('renders placeholder copy and Close button when open', () => {
    render(<CreateLocationModal open onClose={() => {}} />)
    expect(screen.getByText('Create Location')).toBeInTheDocument()
    expect(
      screen.getByText(/Location creation form — coming soon\./i),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Close' })).toBeInTheDocument()
  })

  it('calls onClose when the Close button is clicked', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<CreateLocationModal open onClose={onClose} />)

    await user.click(screen.getByRole('button', { name: 'Close' }))
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('calls onClose when the backdrop is clicked', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    const { container } = render(
      <CreateLocationModal open onClose={onClose} />,
    )

    // Backdrop is the outermost element
    const backdrop = container.firstChild as HTMLElement
    await user.click(backdrop)
    expect(onClose).toHaveBeenCalled()
  })
})
