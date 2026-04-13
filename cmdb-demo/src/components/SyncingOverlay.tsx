import { useState, useEffect } from 'react'

export default function SyncingOverlay() {
  const [visible, setVisible] = useState(false)

  useEffect(() => {
    const handler = () => setVisible(true)
    window.addEventListener('sync-in-progress', handler)
    return () => window.removeEventListener('sync-in-progress', handler)
  }, [])

  useEffect(() => {
    if (!visible) return

    const interval = setInterval(async () => {
      try {
        const res = await fetch('/readyz')
        if (res.ok) {
          setVisible(false)
          window.location.reload()
        }
      } catch {
        // still syncing, retry
      }
    }, 5000)

    return () => clearInterval(interval)
  }, [visible])

  if (!visible) return null

  return (
    <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-[9999]">
      <div className="bg-surface rounded-xl p-8 max-w-md text-center">
        <div className="w-10 h-10 border-3 border-primary/30 border-t-primary rounded-full animate-spin mx-auto mb-4" />
        <h2 className="text-lg font-bold text-on-surface mb-2">Initial Sync in Progress</h2>
        <p className="text-sm text-on-surface-variant">
          This Edge node is synchronizing data from Central for the first time.
          The application will be available once sync completes.
        </p>
        <p className="text-xs text-on-surface-variant mt-4">
          Checking every 5 seconds...
        </p>
      </div>
    </div>
  )
}
