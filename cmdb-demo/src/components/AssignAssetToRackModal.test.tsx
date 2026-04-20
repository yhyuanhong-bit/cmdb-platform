import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

const createSlotMutate = vi.fn()
const createSlotState = { isPending: false }

// Return a stable, realistic-looking assets payload.
const assetsFixture = [
  {
    id: 'a0000000-0000-0000-0000-000000000101',
    name: 'test-server-a',
    asset_tag: 'T-101',
  },
  {
    id: 'a0000000-0000-0000-0000-000000000102',
    name: 'test-server-b',
    asset_tag: 'T-102',
  },
]

vi.mock('../hooks/useAssets', () => ({
  useAssets: () => ({ data: { data: assetsFixture }, isLoading: false }),
}))

vi.mock('../hooks/useTopology', () => ({
  useCreateRackSlot: () => ({
    mutate: createSlotMutate,
    isPending: createSlotState.isPending,
  }),
}))

import AssignAssetToRackModal from './AssignAssetToRackModal'

beforeEach(() => {
  createSlotMutate.mockReset()
  createSlotState.isPending = false
})

describe('AssignAssetToRackModal', () => {
  const rackId = 'a0000000-0000-0000-0000-0000000000aa'

  it('renders nothing when closed', () => {
    const { container } = render(
      <AssignAssetToRackModal
        open={false}
        onClose={() => {}}
        rackId={rackId}
        totalU={42}
      />,
    )
    expect(container.firstChild).toBeNull()
  })

  it('lists all assets from the useAssets hook in the select', () => {
    render(
      <AssignAssetToRackModal
        open
        onClose={() => {}}
        rackId={rackId}
        totalU={42}
      />,
    )

    // The asset-picker select is the first combobox
    const assetSelect = screen.getAllByRole('combobox')[0]
    const options = Array.from(assetSelect.querySelectorAll('option'))
    // Placeholder + 2 assets
    expect(options.length).toBe(3)
    expect(options.some(o => o.textContent === 'test-server-a')).toBe(true)
    expect(options.some(o => o.textContent === 'test-server-b')).toBe(true)
  })

  it('keeps the Assign button disabled until an asset is selected', async () => {
    const user = userEvent.setup()
    render(
      <AssignAssetToRackModal
        open
        onClose={() => {}}
        rackId={rackId}
        totalU={42}
      />,
    )

    const buttons = screen.getAllByRole('button')
    const assignBtn = buttons[buttons.length - 1] as HTMLButtonElement
    expect(assignBtn).toBeDisabled()

    const assetSelect = screen.getAllByRole('combobox')[0]
    await user.selectOptions(assetSelect, assetsFixture[0].id)

    expect(assignBtn).not.toBeDisabled()
  })

  it('calls createRackSlot.mutate with the selected asset and coerced numbers', async () => {
    const user = userEvent.setup()
    render(
      <AssignAssetToRackModal
        open
        onClose={() => {}}
        rackId={rackId}
        totalU={42}
      />,
    )

    const assetSelect = screen.getAllByRole('combobox')[0]
    await user.selectOptions(assetSelect, assetsFixture[1].id)

    const [startU, endU] = screen.getAllByRole('spinbutton')
    // Controlled number input clamps to [1, totalU] on every keystroke, so a
    // plain clear-then-type yields a concatenated value. Select-all-then-type
    // replaces the selection atomically.
    await user.tripleClick(startU)
    await user.keyboard('3')
    await user.tripleClick(endU)
    await user.keyboard('5')

    const buttons = screen.getAllByRole('button')
    const assignBtn = buttons[buttons.length - 1]
    await user.click(assignBtn)

    expect(createSlotMutate).toHaveBeenCalledOnce()
    const [arg] = createSlotMutate.mock.calls[0]
    expect(arg.rackId).toBe(rackId)
    expect(arg.data.asset_id).toBe(assetsFixture[1].id)
    expect(arg.data.start_u).toBe(3)
    expect(arg.data.end_u).toBe(5)
    expect(arg.data.side).toBe('front')
  })

  it('clamps start_u within [1, totalU]', async () => {
    const user = userEvent.setup()
    render(
      <AssignAssetToRackModal
        open
        onClose={() => {}}
        rackId={rackId}
        totalU={10}
      />,
    )

    const [startU] = screen.getAllByRole('spinbutton')
    await user.tripleClick(startU)
    await user.keyboard('99') // above totalU — should clamp to 10 at some keystroke
    // After two digits, onChange sees "99" → Math.min(10, 99) = 10.
    expect(Number((startU as HTMLInputElement).value)).toBeLessThanOrEqual(10)
  })
})
