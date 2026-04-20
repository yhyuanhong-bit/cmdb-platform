import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

const createMutate = vi.fn()
const mutationState = { isPending: false }

vi.mock('../hooks/useIntegration', () => ({
  useCreateWebhook: () => ({
    mutate: createMutate,
    isPending: mutationState.isPending,
  }),
}))

import CreateWebhookModal from './CreateWebhookModal'

beforeEach(() => {
  createMutate.mockReset()
  mutationState.isPending = false
})

describe('CreateWebhookModal', () => {
  it('renders nothing when closed', () => {
    const { container } = render(
      <CreateWebhookModal open={false} onClose={() => {}} />,
    )
    expect(container.firstChild).toBeNull()
  })

  it('disables Create while name is empty or url is empty', () => {
    render(<CreateWebhookModal open onClose={() => {}} />)
    const buttons = screen.getAllByRole('button')
    const createBtn = buttons[buttons.length - 1] as HTMLButtonElement
    expect(createBtn).toBeDisabled()
  })

  it('shows url validation error on blur when url is not http(s)', async () => {
    const user = userEvent.setup()
    render(<CreateWebhookModal open onClose={() => {}} />)

    const inputs = screen.getAllByRole('textbox')
    // name, url, events, secret
    await user.type(inputs[0], 'prod-webhook')
    const urlInput = inputs[1]
    await user.type(urlInput, 'ftp://example.invalid/hook')
    await user.tab() // blur url input — triggers urlTouched

    // aria-invalid is set on the url input and an error description node exists
    expect(urlInput.getAttribute('aria-invalid')).toBe('true')
    expect(document.getElementById('webhook-url-error')).not.toBeNull()
  })

  it('accepts a valid https url and enables the Create button', async () => {
    const user = userEvent.setup()
    render(<CreateWebhookModal open onClose={() => {}} />)

    const inputs = screen.getAllByRole('textbox')
    await user.type(inputs[0], 'prod-webhook')
    await user.type(inputs[1], 'https://example.invalid/webhook')

    const buttons = screen.getAllByRole('button')
    const createBtn = buttons[buttons.length - 1] as HTMLButtonElement
    expect(createBtn).not.toBeDisabled()
  })

  it('splits events CSV into an array on submit', async () => {
    const user = userEvent.setup()
    render(<CreateWebhookModal open onClose={() => {}} />)

    const inputs = screen.getAllByRole('textbox')
    await user.type(inputs[0], 'prod-webhook')
    await user.type(inputs[1], 'https://example.invalid/webhook')
    await user.type(inputs[2], 'asset.created, asset.updated , ,asset.deleted')

    const buttons = screen.getAllByRole('button')
    const createBtn = buttons[buttons.length - 1]
    await user.click(createBtn)

    expect(createMutate).toHaveBeenCalledOnce()
    const [payload] = createMutate.mock.calls[0]
    expect(payload.events).toEqual([
      'asset.created',
      'asset.updated',
      'asset.deleted',
    ])
  })
})
