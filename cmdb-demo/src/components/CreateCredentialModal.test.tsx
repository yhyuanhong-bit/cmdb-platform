import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

const createMutate = vi.fn()
const updateMutate = vi.fn()
const mutationState = { createPending: false, updatePending: false }

vi.mock('../hooks/useCredentials', () => ({
  useCreateCredential: () => ({
    mutate: createMutate,
    isPending: mutationState.createPending,
  }),
  useUpdateCredential: () => ({
    mutate: updateMutate,
    isPending: mutationState.updatePending,
  }),
}))

import CreateCredentialModal from './CreateCredentialModal'

beforeEach(() => {
  createMutate.mockReset()
  updateMutate.mockReset()
  mutationState.createPending = false
  mutationState.updatePending = false
})

describe('CreateCredentialModal', () => {
  it('renders nothing when closed', () => {
    const { container } = render(
      <CreateCredentialModal open={false} onClose={() => {}} />,
    )
    expect(container.firstChild).toBeNull()
  })

  it('defaults to ssh_password type with username and password fields visible', () => {
    render(<CreateCredentialModal open onClose={() => {}} />)
    // ssh_password mode shows username (text) + password (password type)
    const textbox = screen.getAllByRole('textbox')
    // ssh_password shows: name + username = 2 text inputs
    expect(textbox.length).toBe(2)
    // One hidden password input exists for the password field
    const passwordInput = screen.getByPlaceholderText(/password/i)
    expect((passwordInput as HTMLInputElement).type).toBe('password')
  })

  it('switches fields when type changes to snmp_v2c', async () => {
    const user = userEvent.setup()
    render(<CreateCredentialModal open onClose={() => {}} />)

    const typeSelect = screen.getAllByRole('combobox')[0]
    await user.selectOptions(typeSelect, 'snmp_v2c')

    // snmp_v2c shows a community field with placeholder "public"
    expect(screen.getByPlaceholderText('public')).toBeInTheDocument()
  })

  it('switches fields when type changes to ssh_key (renders a textarea for private_key)', async () => {
    const user = userEvent.setup()
    render(<CreateCredentialModal open onClose={() => {}} />)

    const typeSelect = screen.getAllByRole('combobox')[0]
    await user.selectOptions(typeSelect, 'ssh_key')

    // textarea should now be present for private_key
    const textareas = screen.queryAllByRole('textbox').filter(el => el.tagName === 'TEXTAREA')
    expect(textareas.length).toBeGreaterThan(0)
  })

  it('calls createCredential.mutate with built params for ssh_password', async () => {
    const user = userEvent.setup()
    render(<CreateCredentialModal open onClose={() => {}} />)

    // Fill name
    const inputs = screen.getAllByRole('textbox')
    await user.type(inputs[0], 'prod-ssh')
    // username is second text input
    await user.type(inputs[1], 'deployer')
    // password is the password input
    const passwordInput = screen.getByPlaceholderText(/password/i)
    await user.type(passwordInput, 'not-a-real-password')

    // Primary button is the last button
    const buttons = screen.getAllByRole('button')
    const createBtn = buttons[buttons.length - 1]
    await user.click(createBtn)

    expect(createMutate).toHaveBeenCalledOnce()
    const [payload] = createMutate.mock.calls[0]
    expect(payload.name).toBe('prod-ssh')
    expect(payload.type).toBe('ssh_password')
    expect(payload.params.username).toBe('deployer')
    expect(payload.params.password).toBe('not-a-real-password')
  })

  it('calls updateCredential.mutate when editing an existing record', async () => {
    const user = userEvent.setup()
    const editing = {
      id: 'a0000000-0000-0000-0000-000000000042',
      name: 'existing-cred',
      type: 'ssh_password',
      params: { username: 'deployer' },
    }
    render(
      <CreateCredentialModal
        open
        onClose={() => {}}
        editing={editing}
      />,
    )

    // Set a new password so update payload carries it
    const passwordInput = screen.getByPlaceholderText('••••••••')
    await user.type(passwordInput, 'rotated-secret')

    const buttons = screen.getAllByRole('button')
    const saveBtn = buttons[buttons.length - 1]
    await user.click(saveBtn)

    expect(updateMutate).toHaveBeenCalledOnce()
    const [arg] = updateMutate.mock.calls[0]
    expect(arg.id).toBe('a0000000-0000-0000-0000-000000000042')
    expect(arg.data.params.password).toBe('rotated-secret')
  })
})
