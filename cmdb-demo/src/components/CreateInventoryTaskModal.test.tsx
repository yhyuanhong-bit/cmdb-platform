import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

const createTaskMutate = vi.fn()
const mutationState = { isPending: false }

vi.mock('../hooks/useInventory', () => ({
  useCreateInventoryTask: () => ({
    mutate: createTaskMutate,
    isPending: mutationState.isPending,
  }),
}))

import CreateInventoryTaskModal from './CreateInventoryTaskModal'

beforeEach(() => {
  createTaskMutate.mockReset()
  mutationState.isPending = false
})

describe('CreateInventoryTaskModal', () => {
  it('renders nothing when closed', () => {
    const { container } = render(
      <CreateInventoryTaskModal open={false} onClose={() => {}} />,
    )
    expect(container.firstChild).toBeNull()
  })

  it('disables Create while name is empty', () => {
    render(<CreateInventoryTaskModal open onClose={() => {}} />)
    const buttons = screen.getAllByRole('button')
    const createBtn = buttons[buttons.length - 1] as HTMLButtonElement
    expect(createBtn).toBeDisabled()
  })

  it('submits form data with default method=barcode when only name is filled', async () => {
    const user = userEvent.setup()
    render(<CreateInventoryTaskModal open onClose={() => {}} />)

    const inputs = screen.getAllByRole('textbox')
    // name is the first textbox
    await user.type(inputs[0], 'Q2 audit scan')

    const buttons = screen.getAllByRole('button')
    const createBtn = buttons[buttons.length - 1]
    await user.click(createBtn)

    expect(createTaskMutate).toHaveBeenCalledOnce()
    const [payload] = createTaskMutate.mock.calls[0]
    expect(payload.name).toBe('Q2 audit scan')
    expect(payload.method).toBe('barcode')
  })

  it('allows method to be changed via the select', async () => {
    const user = userEvent.setup()
    render(<CreateInventoryTaskModal open onClose={() => {}} />)

    const inputs = screen.getAllByRole('textbox')
    await user.type(inputs[0], 'RFID sweep')

    const methodSelect = screen.getByRole('combobox')
    await user.selectOptions(methodSelect, 'rfid')

    const buttons = screen.getAllByRole('button')
    const createBtn = buttons[buttons.length - 1]
    await user.click(createBtn)

    const [payload] = createTaskMutate.mock.calls[0]
    expect(payload.method).toBe('rfid')
  })

  it('calls onClose on Cancel click', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<CreateInventoryTaskModal open onClose={onClose} />)

    const buttons = screen.getAllByRole('button')
    const cancelBtn = buttons[buttons.length - 2]
    await user.click(cancelBtn)

    expect(onClose).toHaveBeenCalledOnce()
  })
})
