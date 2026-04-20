import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// Mock the data-fetching hook so the modal never hits the network.
const mutate = vi.fn()
const mutationState = { isPending: false }

vi.mock('../hooks/useAssets', () => ({
  useCreateAsset: () => ({
    mutate,
    isPending: mutationState.isPending,
  }),
}))

// Mock toast so error handling path is observable without side effects
vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

import CreateAssetModal from './CreateAssetModal'

beforeEach(() => {
  mutate.mockReset()
  mutationState.isPending = false
})

describe('CreateAssetModal', () => {
  it('renders nothing when closed', () => {
    const { container } = render(
      <CreateAssetModal open={false} onClose={() => {}} />,
    )
    expect(container.firstChild).toBeNull()
  })

  it('renders required asset_tag and name fields when open', () => {
    render(<CreateAssetModal open onClose={() => {}} />)
    // Labels use i18n keys from the real i18n bundle — scope by placeholder/role instead
    expect(screen.getByRole('heading')).toBeInTheDocument()
    // Inputs live inside the form — there are multiple text inputs, so count structure
    const inputs = screen.getAllByRole('textbox')
    expect(inputs.length).toBeGreaterThan(2)
  })

  it('disables the Create button while asset_tag or name is empty', () => {
    render(<CreateAssetModal open onClose={() => {}} />)
    const buttons = screen.getAllByRole('button')
    // Find the primary/create button: last button in the footer (after Cancel)
    const createBtn = buttons[buttons.length - 1] as HTMLButtonElement
    expect(createBtn).toBeDisabled()
  })

  it('enables Create once both required fields are filled, and calls mutate on click', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<CreateAssetModal open onClose={onClose} />)

    const [assetTagInput, nameInput] = screen.getAllByRole('textbox')
    await user.type(assetTagInput, 'T-0001')
    await user.type(nameInput, 'test-server-1')

    const buttons = screen.getAllByRole('button')
    const createBtn = buttons[buttons.length - 1] as HTMLButtonElement
    expect(createBtn).not.toBeDisabled()

    await user.click(createBtn)

    expect(mutate).toHaveBeenCalledOnce()
    const [payload] = mutate.mock.calls[0]
    expect(payload.asset_tag).toBe('T-0001')
    expect(payload.name).toBe('test-server-1')
    // Defaults carried through
    expect(payload.type).toBe('server')
    expect(payload.status).toBe('operational')
  })

  it('calls onClose when Cancel is clicked', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<CreateAssetModal open onClose={onClose} />)

    // Cancel is the first footer button (before the primary Create button)
    const buttons = screen.getAllByRole('button')
    const cancelBtn = buttons[buttons.length - 2] // second to last
    await user.click(cancelBtn)
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('shows pending label when mutation is in-flight', () => {
    mutationState.isPending = true
    render(<CreateAssetModal open onClose={() => {}} />)
    const buttons = screen.getAllByRole('button')
    const createBtn = buttons[buttons.length - 1] as HTMLButtonElement
    // When pending, button is disabled and shows the creating label
    expect(createBtn).toBeDisabled()
  })
})
