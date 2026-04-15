interface Props {
  open: boolean
  onClose: () => void
}

export default function CreateLocationModal({ open, onClose }: Props) {
  if (!open) return null

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-surface rounded-xl p-6 w-96" onClick={e => e.stopPropagation()}>
        <h2 className="text-lg font-bold text-on-surface mb-4">Create Location</h2>
        <p className="text-sm text-on-surface-variant">Location creation form — coming soon.</p>
        <div className="flex justify-end mt-4">
          <button onClick={onClose} className="px-4 py-2 rounded bg-surface-container-high text-on-surface-variant">
            Close
          </button>
        </div>
      </div>
    </div>
  )
}
