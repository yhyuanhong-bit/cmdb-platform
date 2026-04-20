import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18n from '../i18n'
import type { WebhookSubscription } from '../lib/api/integration'

const updateMutate = vi.fn()
const mutationState = { isPending: false }

vi.mock('../hooks/useIntegration', () => ({
  useUpdateWebhook: () => ({
    mutate: updateMutate,
    isPending: mutationState.isPending,
  }),
}))

import EditWebhookModal from './EditWebhookModal'

const baseHook: WebhookSubscription = {
  id: 'a0000000-0000-0000-0000-000000000099',
  name: 'existing-webhook',
  url: 'https://example.invalid/hook',
  events: ['asset.created', 'asset.updated'],
  enabled: true,
  tenant_id: 'a0000000-0000-0000-0000-000000000001',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
} as unknown as WebhookSubscription

beforeEach(async () => {
  updateMutate.mockReset()
  mutationState.isPending = false
  // Pin language to English so label regex queries match deterministic copy.
  await i18n.changeLanguage('en')
})

describe('EditWebhookModal', () => {
  it('renders nothing when no webhook is provided', () => {
    const { container } = render(
      <EditWebhookModal webhook={null} onClose={() => {}} />,
    )
    expect(container.firstChild).toBeNull()
  })

  it('prefills the form from the provided webhook', () => {
    render(<EditWebhookModal webhook={baseHook} onClose={() => {}} />)
    expect(
      (screen.getByDisplayValue('existing-webhook') as HTMLInputElement),
    ).toBeInTheDocument()
    expect(
      (screen.getByDisplayValue('https://example.invalid/hook') as HTMLInputElement),
    ).toBeInTheDocument()
    // events rendered as CSV
    expect(
      screen.getByDisplayValue('asset.created, asset.updated'),
    ).toBeInTheDocument()
  })

  it('closes without a mutate call when nothing has changed', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<EditWebhookModal webhook={baseHook} onClose={onClose} />)

    const buttons = screen.getAllByRole('button')
    const saveBtn = buttons[buttons.length - 1]
    await user.click(saveBtn)

    expect(updateMutate).not.toHaveBeenCalled()
    expect(onClose).toHaveBeenCalled()
  })

  it('sends a patch only for changed fields on save', async () => {
    const user = userEvent.setup()
    render(<EditWebhookModal webhook={baseHook} onClose={() => {}} />)

    const nameInput = screen.getByDisplayValue('existing-webhook') as HTMLInputElement
    await user.clear(nameInput)
    await user.type(nameInput, 'renamed-webhook')

    const buttons = screen.getAllByRole('button')
    const saveBtn = buttons[buttons.length - 1]
    await user.click(saveBtn)

    expect(updateMutate).toHaveBeenCalledOnce()
    const [arg] = updateMutate.mock.calls[0]
    expect(arg.id).toBe(baseHook.id)
    expect(arg.data).toEqual({ name: 'renamed-webhook' })
  })

  it('rotates the secret only when the "rotate secret" checkbox is checked and a new value is provided', async () => {
    const user = userEvent.setup()
    render(<EditWebhookModal webhook={baseHook} onClose={() => {}} />)

    await user.click(screen.getByLabelText(/rotate signing secret/i))

    const secretInput = screen.getByPlaceholderText(/enter new signing secret/i) as HTMLInputElement
    await user.type(secretInput, 'rotated-fake-secret-value')

    const buttons = screen.getAllByRole('button')
    const saveBtn = buttons[buttons.length - 1]
    await user.click(saveBtn)

    expect(updateMutate).toHaveBeenCalledOnce()
    const [arg] = updateMutate.mock.calls[0]
    expect(arg.data.secret).toBe('rotated-fake-secret-value')
  })

  it('marks url as invalid on blur if url is not http(s)', async () => {
    const user = userEvent.setup()
    render(<EditWebhookModal webhook={baseHook} onClose={() => {}} />)

    const urlInput = screen.getByDisplayValue('https://example.invalid/hook') as HTMLInputElement
    await user.clear(urlInput)
    await user.type(urlInput, 'mailto:admin@example.invalid')
    await user.tab()

    expect(urlInput.getAttribute('aria-invalid')).toBe('true')
  })
})
