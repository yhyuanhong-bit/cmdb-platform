import { useRef, useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Modal } from './Modal'

describe('Modal', () => {
  it('renders nothing when closed', () => {
    render(
      <Modal open={false} onOpenChange={() => {}}>
        <Modal.Body>hidden content</Modal.Body>
      </Modal>
    )
    expect(screen.queryByText('hidden content')).not.toBeInTheDocument()
  })

  it('renders via portal on document.body when open', () => {
    render(
      <Modal open onOpenChange={() => {}}>
        <Modal.Body>portaled content</Modal.Body>
      </Modal>
    )
    const content = screen.getByText('portaled content')
    // Must be outside the React root — portaled to body
    expect(content.closest('[data-testid="test-root"]')).toBeNull()
    expect(content).toBeInTheDocument()
  })

  it('applies dialog aria attributes', () => {
    render(
      <Modal open onOpenChange={() => {}}>
        <Modal.Header title="Dialog heading" onClose={() => {}} />
        <Modal.Body>body</Modal.Body>
      </Modal>
    )
    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveAttribute('aria-modal', 'true')
    const headingId = dialog.getAttribute('aria-labelledby')
    expect(headingId).toBeTruthy()
    expect(document.getElementById(headingId!)).toHaveTextContent('Dialog heading')
  })

  it('calls onOpenChange(false) when Escape is pressed', async () => {
    const onOpenChange = vi.fn()
    const user = userEvent.setup()
    render(
      <Modal open onOpenChange={onOpenChange}>
        <Modal.Body>esc content</Modal.Body>
      </Modal>
    )
    await user.keyboard('{Escape}')
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('calls onOpenChange(false) when backdrop is clicked', async () => {
    const onOpenChange = vi.fn()
    const user = userEvent.setup()
    render(
      <Modal open onOpenChange={onOpenChange}>
        <Modal.Body>backdrop content</Modal.Body>
      </Modal>
    )
    const backdrop = screen.getByTestId('modal-backdrop')
    await user.click(backdrop)
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('does not close when content is clicked', async () => {
    const onOpenChange = vi.fn()
    const user = userEvent.setup()
    render(
      <Modal open onOpenChange={onOpenChange}>
        <Modal.Body>
          <p>inner text</p>
        </Modal.Body>
      </Modal>
    )
    await user.click(screen.getByText('inner text'))
    expect(onOpenChange).not.toHaveBeenCalled()
  })

  it('does not close on backdrop click when closeOnBackdropClick=false', async () => {
    const onOpenChange = vi.fn()
    const user = userEvent.setup()
    render(
      <Modal open onOpenChange={onOpenChange} closeOnBackdropClick={false}>
        <Modal.Body>content</Modal.Body>
      </Modal>
    )
    const backdrop = screen.getByTestId('modal-backdrop')
    await user.click(backdrop)
    expect(onOpenChange).not.toHaveBeenCalled()
  })

  it('does not close on Escape when closeOnEscape=false', async () => {
    const onOpenChange = vi.fn()
    const user = userEvent.setup()
    render(
      <Modal open onOpenChange={onOpenChange} closeOnEscape={false}>
        <Modal.Body>content</Modal.Body>
      </Modal>
    )
    await user.keyboard('{Escape}')
    expect(onOpenChange).not.toHaveBeenCalled()
  })

  it('locks body scroll while open and restores on close', () => {
    const { rerender } = render(
      <Modal open onOpenChange={() => {}}>
        <Modal.Body>content</Modal.Body>
      </Modal>
    )
    expect(document.body.style.overflow).toBe('hidden')
    rerender(
      <Modal open={false} onOpenChange={() => {}}>
        <Modal.Body>content</Modal.Body>
      </Modal>
    )
    expect(document.body.style.overflow).not.toBe('hidden')
  })

  it('returns focus to the trigger after close', async () => {
    const user = userEvent.setup()
    function Harness() {
      const [open, setOpen] = useState(false)
      const btnRef = useRef<HTMLButtonElement>(null)
      return (
        <>
          <button ref={btnRef} onClick={() => setOpen(true)}>open-me</button>
          <Modal open={open} onOpenChange={setOpen}>
            <Modal.Body>
              <button onClick={() => setOpen(false)}>close-me</button>
            </Modal.Body>
          </Modal>
        </>
      )
    }
    render(<Harness />)
    const trigger = screen.getByText('open-me')
    trigger.focus()
    await user.click(trigger)
    const close = await screen.findByText('close-me')
    await user.click(close)
    await waitFor(() => expect(document.activeElement).toBe(trigger))
  })

  it('Header renders title and close button, close triggers onOpenChange', async () => {
    const onOpenChange = vi.fn()
    const user = userEvent.setup()
    render(
      <Modal open onOpenChange={onOpenChange}>
        <Modal.Header title="My title" onClose={() => onOpenChange(false)} />
        <Modal.Body>body</Modal.Body>
      </Modal>
    )
    await user.click(screen.getByLabelText(/close/i))
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('supports size variants via data attribute', () => {
    const { rerender } = render(
      <Modal open onOpenChange={() => {}} size="sm">
        <Modal.Body>x</Modal.Body>
      </Modal>
    )
    expect(screen.getByRole('dialog')).toHaveAttribute('data-size', 'sm')
    rerender(
      <Modal open onOpenChange={() => {}} size="lg">
        <Modal.Body>x</Modal.Body>
      </Modal>
    )
    expect(screen.getByRole('dialog')).toHaveAttribute('data-size', 'lg')
  })

  it('Footer renders children in footer region', () => {
    render(
      <Modal open onOpenChange={() => {}}>
        <Modal.Body>body</Modal.Body>
        <Modal.Footer>
          <button>OK</button>
        </Modal.Footer>
      </Modal>
    )
    expect(screen.getByText('OK')).toBeInTheDocument()
  })

  it('traps focus inside the dialog with Tab', async () => {
    const user = userEvent.setup()
    render(
      <>
        <button>outside-before</button>
        <Modal open onOpenChange={() => {}}>
          <Modal.Body>
            <button>first</button>
            <button>last</button>
          </Modal.Body>
        </Modal>
        <button>outside-after</button>
      </>
    )
    const first = screen.getByText('first')
    const last = screen.getByText('last')
    // After open, focus should land inside the dialog
    await waitFor(() => {
      expect([first, last]).toContain(document.activeElement)
    })
    // Tabbing from last should wrap to first, not escape to outside
    last.focus()
    await user.tab()
    expect(document.activeElement).toBe(first)
    // Shift+Tab from first wraps back to last
    first.focus()
    await user.tab({ shift: true })
    expect(document.activeElement).toBe(last)
  })
})
