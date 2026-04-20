import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18n from '../i18n'
import type { AdapterConfig } from '../lib/api/integration'

const updateMutate = vi.fn()
const mutationState = { isPending: false }

vi.mock('../hooks/useIntegration', () => ({
  useUpdateAdapter: () => ({
    mutate: updateMutate,
    isPending: mutationState.isPending,
  }),
}))

import EditAdapterModal from './EditAdapterModal'

const baseAdapter: AdapterConfig = {
  id: 'a0000000-0000-0000-0000-000000000077',
  name: 'prod-adapter',
  type: 'servicenow',
  direction: 'outbound',
  endpoint: 'https://sn.example.invalid/api',
  enabled: true,
}

beforeEach(async () => {
  updateMutate.mockReset()
  mutationState.isPending = false
  await i18n.changeLanguage('en')
})

describe('EditAdapterModal', () => {
  it('renders nothing when adapter is null', () => {
    const { container } = render(
      <EditAdapterModal adapter={null} onClose={() => {}} />,
    )
    expect(container.firstChild).toBeNull()
  })

  it('prefills name, endpoint, and enabled from the provided adapter', () => {
    render(<EditAdapterModal adapter={baseAdapter} onClose={() => {}} />)

    expect(screen.getByDisplayValue('prod-adapter')).toBeInTheDocument()
    expect(
      screen.getByDisplayValue('https://sn.example.invalid/api'),
    ).toBeInTheDocument()
    // type/direction are read-only
    expect(
      (screen.getByDisplayValue('servicenow') as HTMLInputElement).readOnly,
    ).toBe(true)
    expect(
      (screen.getByDisplayValue('outbound') as HTMLInputElement).readOnly,
    ).toBe(true)
  })

  it('closes without calling mutate when nothing has changed', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<EditAdapterModal adapter={baseAdapter} onClose={onClose} />)

    const buttons = screen.getAllByRole('button')
    const saveBtn = buttons[buttons.length - 1]
    await user.click(saveBtn)

    expect(updateMutate).not.toHaveBeenCalled()
    expect(onClose).toHaveBeenCalled()
  })

  it('sends a minimal patch for a changed endpoint', async () => {
    const user = userEvent.setup()
    render(<EditAdapterModal adapter={baseAdapter} onClose={() => {}} />)

    const endpointInput = screen.getByDisplayValue(
      'https://sn.example.invalid/api',
    ) as HTMLInputElement
    await user.clear(endpointInput)
    await user.type(endpointInput, 'https://sn.example.invalid/v2/api')

    const buttons = screen.getAllByRole('button')
    const saveBtn = buttons[buttons.length - 1]
    await user.click(saveBtn)

    expect(updateMutate).toHaveBeenCalledOnce()
    const [arg] = updateMutate.mock.calls[0]
    expect(arg.id).toBe(baseAdapter.id)
    expect(arg.data).toEqual({
      endpoint: 'https://sn.example.invalid/v2/api',
    })
  })

  it('disables save when name is cleared', async () => {
    const user = userEvent.setup()
    render(<EditAdapterModal adapter={baseAdapter} onClose={() => {}} />)

    const nameInput = screen.getByDisplayValue('prod-adapter') as HTMLInputElement
    await user.clear(nameInput)

    const buttons = screen.getAllByRole('button')
    const saveBtn = buttons[buttons.length - 1] as HTMLButtonElement
    expect(saveBtn).toBeDisabled()
  })
})
