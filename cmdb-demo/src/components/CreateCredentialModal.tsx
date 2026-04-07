import { useState, useEffect } from 'react'
import { useCreateCredential, useUpdateCredential } from '../hooks/useCredentials'
import { useAuthStore } from '../stores/authStore'

interface Props {
  open: boolean
  onClose: () => void
  editing?: any
}

const CRED_TYPES = [
  { value: 'snmp_v2c', label: 'SNMP v2c' },
  { value: 'snmp_v3', label: 'SNMP v3' },
  { value: 'ssh_password', label: 'SSH Password' },
  { value: 'ssh_key', label: 'SSH Key' },
  { value: 'ipmi', label: 'IPMI' },
]

const initial = {
  name: '',
  type: 'ssh_password',
  // dynamic fields
  community: '',
  username: '',
  password: '',
  auth_pass: '',
  priv_pass: '',
  auth_proto: 'MD5',
  priv_proto: 'DES',
  private_key: '',
  passphrase: '',
}

export default function CreateCredentialModal({ open, onClose, editing }: Props) {
  const tenantId = useAuthStore((s) => s.tenantId) ?? 'a0000000-0000-0000-0000-000000000001'
  const [formData, setFormData] = useState({ ...initial })
  const createCredential = useCreateCredential()
  const updateCredential = useUpdateCredential()

  useEffect(() => {
    if (editing) {
      setFormData({
        ...initial,
        name: editing.name ?? '',
        type: editing.type ?? 'ssh_password',
        // params come from editing.params – pre-fill non-secret fields
        community: editing.params?.community ?? '',
        username: editing.params?.username ?? '',
        auth_proto: editing.params?.auth_proto ?? 'MD5',
        priv_proto: editing.params?.priv_proto ?? 'DES',
        // secret fields left blank in edit mode (placeholder shows ••••••••)
        password: '',
        auth_pass: '',
        priv_pass: '',
        private_key: '',
        passphrase: '',
      })
    } else {
      setFormData({ ...initial })
    }
  }, [editing, open])

  if (!open) return null

  const set = (key: string, value: string) => setFormData(p => ({ ...p, [key]: value }))

  function buildParams() {
    const t = formData.type
    const params: Record<string, string> = {}

    if (t === 'snmp_v2c') {
      params.community = formData.community
    } else if (t === 'snmp_v3') {
      params.username = formData.username
      params.auth_proto = formData.auth_proto
      params.priv_proto = formData.priv_proto
      if (formData.auth_pass) params.auth_pass = formData.auth_pass
      if (formData.priv_pass) params.priv_pass = formData.priv_pass
    } else if (t === 'ssh_password') {
      params.username = formData.username
      if (formData.password) params.password = formData.password
    } else if (t === 'ssh_key') {
      params.username = formData.username
      if (formData.private_key) params.private_key = formData.private_key
      if (formData.passphrase) params.passphrase = formData.passphrase
    } else if (t === 'ipmi') {
      params.username = formData.username
      if (formData.password) params.password = formData.password
    }

    return params
  }

  function handleSubmit() {
    const payload = {
      tenant_id: tenantId,
      name: formData.name,
      type: formData.type,
      params: buildParams(),
    }

    if (editing) {
      updateCredential.mutate(
        { id: editing.id, data: payload },
        { onSuccess: () => { onClose(); setFormData({ ...initial }) } }
      )
    } else {
      createCredential.mutate(payload, {
        onSuccess: () => { onClose(); setFormData({ ...initial }) },
      })
    }
  }

  const isPending = createCredential.isPending || updateCredential.isPending
  const isEdit = !!editing

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div
        className="bg-[#1a1f2e] p-6 rounded-xl w-[28rem] space-y-4 max-h-[90vh] overflow-y-auto"
        onClick={e => e.stopPropagation()}
      >
        <h3 className="text-lg font-bold text-white">
          {isEdit ? 'Edit Credential' : 'Create Credential'}
        </h3>

        {/* Name */}
        <div>
          <label className="block text-sm text-gray-400 mb-1">Name *</label>
          <input
            value={formData.name}
            onChange={e => set('name', e.target.value)}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
            placeholder="e.g. datacenter-snmp-ro"
          />
        </div>

        {/* Type */}
        <div>
          <label className="block text-sm text-gray-400 mb-1">Type</label>
          <select
            value={formData.type}
            onChange={e => set('type', e.target.value)}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
          >
            {CRED_TYPES.map(ct => (
              <option key={ct.value} value={ct.value}>{ct.label}</option>
            ))}
          </select>
        </div>

        {/* Dynamic fields: snmp_v2c */}
        {formData.type === 'snmp_v2c' && (
          <div>
            <label className="block text-sm text-gray-400 mb-1">Community String *</label>
            <input
              value={formData.community}
              onChange={e => set('community', e.target.value)}
              className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
              placeholder="public"
            />
          </div>
        )}

        {/* Dynamic fields: snmp_v3 */}
        {formData.type === 'snmp_v3' && (
          <>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Username *</label>
              <input
                value={formData.username}
                onChange={e => set('username', e.target.value)}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
                placeholder="snmpuser"
              />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Auth Password{isEdit && ' (leave blank to keep)'}</label>
              <input
                type="password"
                value={formData.auth_pass}
                onChange={e => set('auth_pass', e.target.value)}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
                placeholder={isEdit ? '••••••••' : 'Auth password'}
              />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Priv Password{isEdit && ' (leave blank to keep)'}</label>
              <input
                type="password"
                value={formData.priv_pass}
                onChange={e => set('priv_pass', e.target.value)}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
                placeholder={isEdit ? '••••••••' : 'Priv password'}
              />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Auth Protocol</label>
              <select
                value={formData.auth_proto}
                onChange={e => set('auth_proto', e.target.value)}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
              >
                <option value="MD5">MD5</option>
                <option value="SHA">SHA</option>
              </select>
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Priv Protocol</label>
              <select
                value={formData.priv_proto}
                onChange={e => set('priv_proto', e.target.value)}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
              >
                <option value="DES">DES</option>
                <option value="AES">AES</option>
              </select>
            </div>
          </>
        )}

        {/* Dynamic fields: ssh_password / ipmi */}
        {(formData.type === 'ssh_password' || formData.type === 'ipmi') && (
          <>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Username *</label>
              <input
                value={formData.username}
                onChange={e => set('username', e.target.value)}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
                placeholder="root"
              />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Password{isEdit && ' (leave blank to keep)'} *</label>
              <input
                type="password"
                value={formData.password}
                onChange={e => set('password', e.target.value)}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
                placeholder={isEdit ? '••••••••' : 'Password'}
              />
            </div>
          </>
        )}

        {/* Dynamic fields: ssh_key */}
        {formData.type === 'ssh_key' && (
          <>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Username *</label>
              <input
                value={formData.username}
                onChange={e => set('username', e.target.value)}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
                placeholder="root"
              />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Private Key{isEdit && ' (leave blank to keep)'} *</label>
              <textarea
                value={formData.private_key}
                onChange={e => set('private_key', e.target.value)}
                rows={5}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm font-mono resize-y"
                placeholder={isEdit ? '••••••••' : '-----BEGIN RSA PRIVATE KEY-----\n...'}
              />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Passphrase (optional)</label>
              <input
                type="password"
                value={formData.passphrase}
                onChange={e => set('passphrase', e.target.value)}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
                placeholder={isEdit ? '••••••••' : 'Leave blank if none'}
              />
            </div>
          </>
        )}

        <div className="flex gap-2 justify-end pt-2">
          <button onClick={onClose} className="px-4 py-2 rounded bg-gray-700 text-white text-sm">
            Cancel
          </button>
          <button
            onClick={handleSubmit}
            disabled={isPending || !formData.name}
            className="px-4 py-2 rounded bg-blue-600 text-white text-sm disabled:opacity-50"
          >
            {isPending ? (isEdit ? 'Saving...' : 'Creating...') : isEdit ? 'Save' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  )
}
