import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// Intercept Html5Qrcode before the SUT imports it. vi.mock is hoisted above
// normal imports, so we must also define the mock's externalized state via
// vi.hoisted() to survive the hoist.
const qrMock = vi.hoisted(() => {
  const lastSuccess: { fn: ((text: string) => void) | null } = { fn: null }
  const startMock = (() => Promise.resolve()) as (() => Promise<void>)
  const stopMock = vi.fn(() => Promise.resolve())
  class MockHtml5Qrcode {
    start(
      _cameraConfig: unknown,
      _scanConfig: unknown,
      onSuccess: (text: string) => void,
      _onError: (msg: string) => void,
    ) {
      lastSuccess.fn = onSuccess
      return startMock()
    }
    stop() {
      return stopMock()
    }
  }
  return { lastSuccess, stopMock, MockHtml5Qrcode }
})

vi.mock('html5-qrcode', () => ({
  Html5Qrcode: qrMock.MockHtml5Qrcode,
}))

import QRScanner from './QRScanner'

beforeEach(() => {
  qrMock.lastSuccess.fn = null
  qrMock.stopMock.mockClear()
})

describe('QRScanner', () => {
  it('renders the QR reader container and close button', () => {
    render(<QRScanner onScan={() => {}} onClose={() => {}} />)
    expect(document.getElementById('qr-reader')).not.toBeNull()
    // Icon-only close button shows the material symbol text "close"
    expect(screen.getByText('close')).toBeInTheDocument()
  })

  it('invokes onClose when the close button is clicked', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<QRScanner onScan={() => {}} onClose={onClose} />)

    const closeBtn = screen.getAllByRole('button')[0]
    await user.click(closeBtn)
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('parses JSON QR payloads and forwards typed data to onScan', () => {
    const onScan = vi.fn()
    render(<QRScanner onScan={onScan} onClose={() => {}} />)

    const payload = {
      t: 'asset',
      id: 'a0000000-0000-0000-0000-000000000555',
      tag: 'T-555',
      name: 'test-asset-555',
    }
    expect(qrMock.lastSuccess.fn).not.toBeNull()
    qrMock.lastSuccess.fn!(JSON.stringify(payload))

    expect(onScan).toHaveBeenCalledWith(payload)
  })

  it('treats non-JSON text as a plain asset tag', () => {
    const onScan = vi.fn()
    render(<QRScanner onScan={onScan} onClose={() => {}} />)

    qrMock.lastSuccess.fn!('BARCODE-12345')

    expect(onScan).toHaveBeenCalledWith({
      t: 'asset',
      id: '',
      tag: 'BARCODE-12345',
      name: 'BARCODE-12345',
    })
  })

  it('ignores JSON payloads with an unknown "t" field', () => {
    const onScan = vi.fn()
    render(<QRScanner onScan={onScan} onClose={() => {}} />)

    qrMock.lastSuccess.fn!(JSON.stringify({ t: 'unknown', id: 'x' }))

    expect(onScan).not.toHaveBeenCalled()
  })
})
