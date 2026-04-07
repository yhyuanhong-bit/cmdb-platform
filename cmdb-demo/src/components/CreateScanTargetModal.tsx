import { useState, useEffect } from 'react'
import { useCreateScanTarget, useUpdateScanTarget } from '../hooks/useScanTargets'
import { useCredentials } from '../hooks/useCredentials'

interface ScanTarget {
  id: string
  name: string
  collector_type: string
  cidrs: string[]
  credential_id: string
  mode: string
  tenant_id?: string
}

interface Props {
  open: boolean
  onClose: () => void
  editing?: ScanTarget | null
}

const CREDENTIAL_TYPE_MAP: Record<string, string[]> = {
  snmp:  ['snmp_v2c', 'snmp_v3'],
  ssh:   ['ssh_password', 'ssh_key'],
  ipmi:  ['ipmi'],
}

const initial = {
  name: '',
  collector_type: 'snmp',
  cidrs: '',
  credential_id: '',
  mode: 'auto',
}

export default function CreateScanTargetModal({ open, onClose, editing }: Props) {
  const tenantId = 'a0000000-0000-0000-0000-000000000001'
  const [formData, setFormData] = useState({ ...initial })

  const createMutation = useCreateScanTarget()
  const updateMutation = useUpdateScanTarget()
  const { data: credsData } = useCredentials()

  const credentials: any[] = (credsData as any)?.data ?? []

  /* Populate form when editing */
  useEffect(() => {
    if (editing) {
      setFormData({
        name:           editing.name,
        collector_type: editing.collector_type,
        cidrs:          Array.isArray(editing.cidrs) ? editing.cidrs.join('\n') : '',
        credential_id:  editing.credential_id,
        mode:           editing.mode,
      })
    } else {
      setFormData({ ...initial })
    }
  }, [editing, open])

  if (!open) return null

  const compatibleTypes = CREDENTIAL_TYPE_MAP[formData.collector_type] ?? []
  const filteredCreds = credentials.filter(c => compatibleTypes.includes(c.cred_type))

  const isEditing = Boolean(editing)
  const isPending = createMutation.isPending || updateMutation.isPending

  function handleSubmit() {
    const payload = {
      name:           formData.name,
      collector_type: formData.collector_type,
      cidrs:          formData.cidrs.split('\n').map(s => s.trim()).filter(Boolean),
      credential_id:  formData.credential_id,
      mode:           formData.mode,
      tenant_id:      tenantId,
    }

    if (isEditing && editing) {
      updateMutation.mutate(
        { id: editing.id, data: payload },
        {
          onSuccess: () => {
            onClose()
            setFormData({ ...initial })
          },
        }
      )
    } else {
      createMutation.mutate(payload as any, {
        onSuccess: () => {
          onClose()
          setFormData({ ...initial })
        },
      })
    }
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div
        className="bg-[#1a1f2e] p-6 rounded-xl w-[30rem] space-y-4 max-h-[90vh] overflow-y-auto"
        onClick={e => e.stopPropagation()}
      >
        <h3 className="text-lg font-bold text-white">
          {isEditing ? 'Edit Scan Target' : 'Add Scan Target'}
        </h3>

        {/* Name */}
        <div>
          <label className="block text-sm text-gray-400 mb-1">Name *</label>
          <input
            value={formData.name}
            onChange={e => setFormData(p => ({ ...p, name: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
            placeholder="e.g. Core Network SNMP"
          />
        </div>

        {/* Collector Type */}
        <div>
          <label className="block text-sm text-gray-400 mb-1">Collector Type</label>
          <select
            value={formData.collector_type}
            onChange={e => setFormData(p => ({ ...p, collector_type: e.target.value, credential_id: '' }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
          >
            <option value="snmp">SNMP</option>
            <option value="ssh">SSH</option>
            <option value="ipmi">IPMI</option>
          </select>
        </div>

        {/* CIDRs */}
        <div>
          <label className="block text-sm text-gray-400 mb-1">CIDRs (one per line)</label>
          <textarea
            value={formData.cidrs}
            onChange={e => setFormData(p => ({ ...p, cidrs: e.target.value }))}
            rows={4}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm font-mono resize-y"
            placeholder={"192.168.1.0/24\n10.0.0.0/8"}
          />
        </div>

        {/* Credential */}
        <div>
          <label className="block text-sm text-gray-400 mb-1">Credential</label>
          <select
            value={formData.credential_id}
            onChange={e => setFormData(p => ({ ...p, credential_id: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
          >
            <option value="">— Select credential —</option>
            {filteredCreds.map((c: any) => (
              <option key={c.id} value={c.id}>
                {c.name} ({c.cred_type})
              </option>
            ))}
          </select>
          {filteredCreds.length === 0 && (
            <p className="text-xs text-gray-500 mt-1">
              No compatible credentials found for {formData.collector_type.toUpperCase()}.
            </p>
          )}
        </div>

        {/* Mode */}
        <div>
          <label className="block text-sm text-gray-400 mb-1">Mode</label>
          <select
            value={formData.mode}
            onChange={e => setFormData(p => ({ ...p, mode: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
          >
            <option value="auto">Auto</option>
            <option value="review">Review</option>
            <option value="smart">Smart</option>
          </select>
        </div>

        {/* Buttons */}
        <div className="flex gap-2 justify-end pt-2">
          <button
            onClick={onClose}
            className="px-4 py-2 rounded bg-gray-700 text-white text-sm hover:bg-gray-600 transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleSubmit}
            disabled={isPending || !formData.name}
            className="px-4 py-2 rounded bg-blue-600 text-white text-sm disabled:opacity-50 hover:bg-blue-500 transition-colors"
          >
            {isPending ? (isEditing ? 'Saving...' : 'Creating...') : (isEditing ? 'Save Changes' : 'Create')}
          </button>
        </div>
      </div>
    </div>
  )
}
