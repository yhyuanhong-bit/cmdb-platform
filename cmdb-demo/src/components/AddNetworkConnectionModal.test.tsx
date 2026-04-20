import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

const createConnMutate = vi.fn()
const mutationState = { isPending: false, isError: false }

vi.mock('../hooks/useTopology', () => ({
  useCreateNetworkConnection: () => ({
    mutate: createConnMutate,
    isPending: mutationState.isPending,
    isError: mutationState.isError,
  }),
}))

const assetsFixture = [
  {
    id: 'a0000000-0000-0000-0000-000000000201',
    name: 'switch-core-test-1',
    asset_tag: 'SW-201',
  },
]

vi.mock('../hooks/useAssets', () => ({
  useAssets: () => ({ data: { data: assetsFixture }, isLoading: false }),
}))

import AddNetworkConnectionModal from './AddNetworkConnectionModal'

beforeEach(() => {
  createConnMutate.mockReset()
  mutationState.isPending = false
  mutationState.isError = false
})

describe('AddNetworkConnectionModal', () => {
  const rackId = 'a0000000-0000-0000-0000-0000000000bb'

  it('renders nothing when closed', () => {
    const { container } = render(
      <AddNetworkConnectionModal
        open={false}
        onClose={() => {}}
        rackId={rackId}
      />,
    )
    expect(container.firstChild).toBeNull()
  })

  it('shows an asset dropdown when deviceType is internal (default)', () => {
    render(
      <AddNetworkConnectionModal open onClose={() => {}} rackId={rackId} />,
    )
    // The internal asset select is present
    expect(screen.getByText('— Select Asset —')).toBeInTheDocument()
    // External device text input should NOT be present
    expect(
      screen.queryByPlaceholderText(/switch-core-01/i),
    ).not.toBeInTheDocument()
  })

  it('swaps to external device text input when deviceType is set to external', async () => {
    const user = userEvent.setup()
    render(
      <AddNetworkConnectionModal open onClose={() => {}} rackId={rackId} />,
    )

    const externalRadio = screen.getByRole('radio', { name: /external/i })
    await user.click(externalRadio)

    expect(
      screen.getByPlaceholderText(/switch-core-01/i),
    ).toBeInTheDocument()
    // Internal select is gone
    expect(screen.queryByText('— Select Asset —')).not.toBeInTheDocument()
  })

  it('parses VLAN CSV into a numeric array on submit (ignores NaN)', async () => {
    const user = userEvent.setup()
    render(
      <AddNetworkConnectionModal open onClose={() => {}} rackId={rackId} />,
    )

    // Port input
    const portInput = screen.getByPlaceholderText('Eth1/1') as HTMLInputElement
    await user.type(portInput, 'Eth1/4')

    // Select internal asset
    const assetSelect = screen.getAllByRole('combobox')[0]
    await user.selectOptions(assetSelect, assetsFixture[0].id)

    // VLANs — mix of valid and invalid
    const vlansInput = screen.getByPlaceholderText('100,200,300') as HTMLInputElement
    await user.type(vlansInput, '100, abc,200')

    // Submit button — first button in the footer (type=submit)
    const submitBtn = screen.getByRole('button', { name: /add connection|\.\.\./i })
    await user.click(submitBtn)

    expect(createConnMutate).toHaveBeenCalledOnce()
    const [arg] = createConnMutate.mock.calls[0]
    expect(arg.rackId).toBe(rackId)
    expect(arg.data.source_port).toBe('Eth1/4')
    expect(arg.data.vlans).toEqual([100, 200])
    expect(arg.data.connected_asset_id).toBe(assetsFixture[0].id)
  })

  it('calls onClose when Cancel is clicked', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(
      <AddNetworkConnectionModal open onClose={onClose} rackId={rackId} />,
    )

    await user.click(screen.getByRole('button', { name: /cancel/i }))
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('shows an error message when mutation reports isError', () => {
    mutationState.isError = true
    render(
      <AddNetworkConnectionModal open onClose={() => {}} rackId={rackId} />,
    )
    expect(
      screen.getByText(/failed to create connection/i),
    ).toBeInTheDocument()
  })
})
