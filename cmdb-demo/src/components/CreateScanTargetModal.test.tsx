import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

const createMutate = vi.fn()
const updateMutate = vi.fn()
const mutationState = { createPending: false, updatePending: false }

// Credentials fixture covers all collector_type → cred_type mappings.
const credentialsFixture = [
  {
    id: 'a0000000-0000-0000-0000-000000000301',
    name: 'snmp-cred-v2c',
    type: 'snmp_v2c',
    cred_type: 'snmp_v2c',
  },
  {
    id: 'a0000000-0000-0000-0000-000000000302',
    name: 'ssh-cred-pw',
    type: 'ssh_password',
    cred_type: 'ssh_password',
  },
  {
    id: 'a0000000-0000-0000-0000-000000000303',
    name: 'ipmi-cred',
    type: 'ipmi',
    cred_type: 'ipmi',
  },
]

vi.mock('../hooks/useCredentials', () => ({
  useCredentials: () => ({
    data: { data: credentialsFixture },
    isLoading: false,
  }),
}))

vi.mock('../hooks/useScanTargets', () => ({
  useCreateScanTarget: () => ({
    mutate: createMutate,
    isPending: mutationState.createPending,
  }),
  useUpdateScanTarget: () => ({
    mutate: updateMutate,
    isPending: mutationState.updatePending,
  }),
}))

import CreateScanTargetModal from './CreateScanTargetModal'

beforeEach(() => {
  createMutate.mockReset()
  updateMutate.mockReset()
  mutationState.createPending = false
  mutationState.updatePending = false
})

describe('CreateScanTargetModal', () => {
  it('renders nothing when closed', () => {
    const { container } = render(
      <CreateScanTargetModal open={false} onClose={() => {}} />,
    )
    expect(container.firstChild).toBeNull()
  })

  it('filters credentials to those matching the selected collector type (default: snmp)', () => {
    render(<CreateScanTargetModal open onClose={() => {}} />)
    // Order of <select>s in the form: collector_type, credential_id, mode
    const credentialSelect = screen.getAllByRole('combobox')[1]
    const options = Array.from(credentialSelect.querySelectorAll('option'))
    // Placeholder + one matching snmp_v2c cred
    expect(options.length).toBe(2)
    expect(options.some(o => o.textContent?.includes('snmp-cred-v2c'))).toBe(true)
  })

  it('re-filters the credential list when collector_type changes to ssh', async () => {
    const user = userEvent.setup()
    render(<CreateScanTargetModal open onClose={() => {}} />)

    const collectorSelect = screen.getAllByRole('combobox')[0]
    await user.selectOptions(collectorSelect, 'ssh')

    const credentialSelect = screen.getAllByRole('combobox')[1]
    const options = Array.from(credentialSelect.querySelectorAll('option'))
    expect(options.some(o => o.textContent?.includes('ssh-cred-pw'))).toBe(true)
    expect(options.some(o => o.textContent?.includes('snmp-cred-v2c'))).toBe(false)
  })

  it('submits a create request with CIDRs split by newline and trimmed', async () => {
    const user = userEvent.setup()
    render(<CreateScanTargetModal open onClose={() => {}} />)

    // name
    const nameInput = screen.getAllByRole('textbox')[0] // first text input
    await user.type(nameInput, 'branch-snmp-scan')

    // cidrs textarea — find by the i18n placeholder (may vary by locale)
    const cidrsArea = screen
      .getAllByRole('textbox')
      .find(el => el.tagName === 'TEXTAREA') as HTMLTextAreaElement
    await user.type(cidrsArea, '10.0.0.0/24\n192.0.2.0/24\n   ')

    const credentialSelect = screen.getAllByRole('combobox')[1]
    await user.selectOptions(credentialSelect, credentialsFixture[0].id)

    const buttons = screen.getAllByRole('button')
    const submitBtn = buttons[buttons.length - 1]
    await user.click(submitBtn)

    expect(createMutate).toHaveBeenCalledOnce()
    const [payload] = createMutate.mock.calls[0]
    expect(payload.name).toBe('branch-snmp-scan')
    expect(payload.collector_type).toBe('snmp')
    expect(payload.cidrs).toEqual(['10.0.0.0/24', '192.0.2.0/24'])
    expect(payload.credential_id).toBe(credentialsFixture[0].id)
  })

  it('routes to updateMutate when editing an existing scan target', async () => {
    const user = userEvent.setup()
    const editing = {
      id: 'a0000000-0000-0000-0000-000000000999',
      name: 'existing-target',
      collector_type: 'snmp',
      cidrs: ['10.0.0.0/24'],
      credential_id: credentialsFixture[0].id,
      mode: 'auto',
    }
    render(
      <CreateScanTargetModal open onClose={() => {}} editing={editing} />,
    )

    // Change the name so submit fires a non-empty update
    const nameInput = screen.getByDisplayValue('existing-target') as HTMLInputElement
    await user.clear(nameInput)
    await user.type(nameInput, 'renamed-target')

    const buttons = screen.getAllByRole('button')
    const saveBtn = buttons[buttons.length - 1]
    await user.click(saveBtn)

    expect(updateMutate).toHaveBeenCalledOnce()
    const [arg] = updateMutate.mock.calls[0]
    expect(arg.id).toBe(editing.id)
    expect(arg.data.name).toBe('renamed-target')
  })
})
