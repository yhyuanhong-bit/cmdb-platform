import { Modal } from './ui/Modal'

interface Props {
  open: boolean
  onClose: () => void
}

export default function CreateLocationModal({ open, onClose }: Props) {
  return (
    <Modal
      open={open}
      onOpenChange={(next) => { if (!next) onClose() }}
      size="sm"
      panelClassName="bg-surface text-on-surface"
    >
      <Modal.Header title="Create Location" onClose={onClose} />
      <Modal.Body>
        <p className="text-sm text-on-surface-variant">Location creation form — coming soon.</p>
      </Modal.Body>
      <Modal.Footer>
        <button
          onClick={onClose}
          className="px-4 py-2 rounded bg-surface-container-high text-on-surface-variant"
        >
          Close
        </button>
      </Modal.Footer>
    </Modal>
  )
}
