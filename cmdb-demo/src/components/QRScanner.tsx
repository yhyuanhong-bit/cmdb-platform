import { useEffect, useRef, useState } from 'react'
import { Html5Qrcode } from 'html5-qrcode'
import { useTranslation } from 'react-i18next'

interface QRData {
  t: 'asset' | 'rack'
  id: string
  tag?: string
  sn?: string
  name?: string
  loc?: string
}

interface QRScannerProps {
  onScan: (data: QRData) => void
  onClose: () => void
}

export default function QRScanner({ onScan, onClose }: QRScannerProps) {
  const { t } = useTranslation()
  const [error, setError] = useState<string | null>(null)
  const scannerRef = useRef<Html5Qrcode | null>(null)

  useEffect(() => {
    const scanner = new Html5Qrcode('qr-reader')
    scannerRef.current = scanner

    scanner.start(
      { facingMode: 'environment' },
      { fps: 10, qrbox: { width: 250, height: 250 } },
      (decodedText) => {
        try {
          const data = JSON.parse(decodedText) as QRData
          if (data.t === 'asset' || data.t === 'rack') {
            scanner.stop().catch(() => {})
            onScan(data)
          }
        } catch {
          // Not a valid QR JSON -- try as plain asset tag
          scanner.stop().catch(() => {})
          onScan({ t: 'asset', id: '', tag: decodedText, name: decodedText })
        }
      },
      () => {} // ignore scan failures (normal while scanning)
    ).catch((err: unknown) => {
      setError(String(err))
    })

    return () => {
      scanner.stop().catch(() => {})
    }
  }, [onScan])

  return (
    <div className="fixed inset-0 z-50 bg-black/80 flex items-center justify-center">
      <div className="bg-surface rounded-2xl p-6 max-w-sm w-full">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-headline font-bold text-on-surface">
            {t('qr.scan_title', 'Scan QR Code')}
          </h3>
          <button onClick={onClose} className="text-on-surface-variant hover:text-on-surface">
            <span className="material-symbols-outlined">close</span>
          </button>
        </div>

        <div id="qr-reader" className="w-full rounded-lg overflow-hidden" />

        {error && (
          <p className="text-error text-xs mt-3">{error}</p>
        )}

        <p className="text-on-surface-variant text-xs mt-3 text-center">
          {t('qr.scan_hint', 'Point camera at the QR code on the device or rack')}
        </p>
      </div>
    </div>
  )
}
