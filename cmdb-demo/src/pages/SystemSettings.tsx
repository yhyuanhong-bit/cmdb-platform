import { toast } from 'sonner'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { useUsers, useRoles, useCreateUser, useUpdateUser, useDeleteUser, useAssignRole } from '../hooks/useIdentity'
import { usePermission } from '../hooks/usePermission'
import { useSystemHealth } from '../hooks/useSystemHealth'
import { useAdapters, useWebhooks } from '../hooks/useIntegration'
import { useCredentials, useDeleteCredential } from '../hooks/useCredentials'
import CreateAdapterModal from '../components/CreateAdapterModal'
import CreateWebhookModal from '../components/CreateWebhookModal'
import CreateCredentialModal from '../components/CreateCredentialModal'

export default function SystemSettings() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const canManageUsers = usePermission('identity', 'write')
  const [activeTab, setActiveTab] = useState('permissions')
  const [showUserModal, setShowUserModal] = useState(false)
  const [editingUser, setEditingUser] = useState<any>(null)
  const [newUserData, setNewUserData] = useState({ username: '', display_name: '', email: '', password: '', role_id: '' })
  const createUser = useCreateUser()
  const updateUser = useUpdateUser()
  const deleteUser = useDeleteUser()
  const assignRole = useAssignRole()
  const [showCreateAdapter, setShowCreateAdapter] = useState(false)
  const [showCreateWebhook, setShowCreateWebhook] = useState(false)
  const [showCreateCredential, setShowCreateCredential] = useState(false)
  const [editingCredential, setEditingCredential] = useState<any>(null)

  const { data: usersResp, isLoading: usersLoading } = useUsers()
  const { data: rolesResp } = useRoles()
  const { data: healthResp } = useSystemHealth()
  const { data: adaptersResp } = useAdapters()
  const { data: webhooksResp } = useWebhooks()
  const { data: credentialsResp } = useCredentials()
  const deleteCredential = useDeleteCredential()

  const apiUsers = (usersResp as any)?.data ?? []
  const apiRoles = (rolesResp as any)?.data ?? []
  const health = (healthResp as any)?.data
  const adapters = (adaptersResp as any)?.data ?? []
  const webhooks = (webhooksResp as any)?.data ?? []
  const credentials = (credentialsResp as any)?.data ?? []

  // Map API users to display format
  const users = apiUsers.map((u: any) => ({
    id: u.id,
    name: u.display_name,
    email: u.email,
    username: u.username,
    role: u.roles?.[0]?.name ?? u.source ?? 'local',
    roleId: u.roles?.[0]?.id ?? '',
    region: 'TW',
    status: u.status === 'active' ? 'Active' : 'Inactive',
    avatar: u.display_name?.split(' ').map((w: any) => w[0]).join('').slice(0, 2).toUpperCase() ?? '??',
    _raw: u,
  }))

  // Map health check to display format
  const healthIndicators = [
    { label: 'Database', status: health?.database?.status ?? 'unknown', latency: `${health?.database?.latency_ms?.toFixed(0) ?? '?'}ms` },
    { label: 'Redis', status: health?.redis?.status ?? 'unknown', latency: `${health?.redis?.latency_ms?.toFixed(0) ?? '?'}ms` },
    { label: 'Event Bus (NATS)', status: health?.nats?.connected ? 'operational' : 'unknown', latency: '-' },
    { label: 'Auth Service', status: 'operational', latency: '-' },
  ]

  return (
    <div className="min-h-screen bg-surface p-6 font-body text-on-surface">
      {/* Breadcrumb */}
      <nav
        aria-label="Breadcrumb"
        className="flex items-center gap-1.5 text-xs uppercase tracking-widest text-on-surface-variant mb-4"
      >
        <span
          className="cursor-pointer transition-colors hover:text-primary"
          onClick={() => navigate('/system')}
        >
          {t('system_settings.breadcrumb_system')}
        </span>
        <span className="text-[10px] opacity-40" aria-hidden="true">›</span>
        <span className="text-on-surface font-semibold">{t('system_settings.title_zh')}</span>
      </nav>

      {/* Title */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="font-headline font-bold text-2xl text-on-surface">{t('system_settings.title_zh')}</h1>
          <p className="text-on-surface-variant text-sm mt-1">{t('system_settings.title')}</p>
        </div>
        {canManageUsers && (
          <button
            onClick={() => { setEditingUser(null); setNewUserData({ username: '', display_name: '', email: '', password: '', role_id: '' }); setShowUserModal(true) }}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg bg-on-primary-container text-white text-sm font-semibold hover:bg-on-primary-container/90 transition-colors"
          >
            <span className="material-symbols-outlined text-[18px]">person_add</span>
            {t('system_settings.btn_new_user')}
          </button>
        )}
      </div>

      {/* Tabs */}
      <div className="flex gap-1 mb-6">
        {['permissions', 'security', 'integrations', 'credentials'].map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-4 py-2 rounded-lg text-sm font-semibold transition-colors ${
              activeTab === tab
                ? 'bg-surface-container-high text-on-surface'
                : 'text-on-surface-variant hover:bg-surface-container'
            }`}
          >
            {tab === 'permissions'
              ? t('system_settings.tab_permissions_matrix')
              : tab === 'security'
              ? t('system_settings.tab_security')
              : tab === 'integrations'
              ? t('system_settings.tab_integrations')
              : t('system_settings.tab_credentials')}
          </button>
        ))}
      </div>

      {activeTab === 'credentials' ? (
        <div className="bg-surface-container rounded-lg p-6">
          <div className="flex items-center justify-between mb-4">
            <h2 className="font-headline font-bold text-lg text-on-surface">{t('system_settings.section_credentials')}</h2>
            <button
              onClick={() => { setEditingCredential(null); setShowCreateCredential(true) }}
              className="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-on-primary-container text-white text-sm font-semibold hover:bg-on-primary-container/90 transition-colors"
            >
              <span className="material-symbols-outlined text-[16px]">add</span>
              {t('system_settings.btn_add_credential')}
            </button>
          </div>

          {/* Table header */}
          <div className="grid grid-cols-[2fr_1fr_1fr_auto] gap-3 px-3 py-2 mb-1">
            <span className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label">{t('system_settings.col_name')}</span>
            <span className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label">{t('system_settings.col_type')}</span>
            <span className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label">{t('system_settings.col_created')}</span>
            <span className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label">{t('system_settings.col_actions')}</span>
          </div>

          {credentials.length === 0 && (
            <p className="text-sm text-on-surface-variant px-3 py-4">{t('system_settings.no_credentials')}</p>
          )}

          {credentials.map((cred: any, i: number) => (
            <div
              key={cred.id}
              className={`grid grid-cols-[2fr_1fr_1fr_auto] gap-3 items-center px-3 py-3 rounded-lg ${
                i % 2 === 0 ? 'bg-surface-container-low' : ''
              }`}
            >
              <span className="text-sm text-on-surface font-semibold">{cred.name}</span>
              <span className="text-xs text-on-surface-variant">{cred.type}</span>
              <span className="text-xs text-on-surface-variant">
                {cred.created_at ? new Date(cred.created_at).toLocaleDateString() : '-'}
              </span>
              <div className="flex gap-1">
                <button
                  onClick={() => { setEditingCredential(cred); setShowCreateCredential(true) }}
                  className="p-1.5 rounded hover:bg-surface-container-high transition-colors"
                >
                  <span className="material-symbols-outlined text-on-surface-variant text-[18px]">edit</span>
                </button>
                <button
                  onClick={() => {
                    if (confirm(t('system_settings.confirm_delete_credential', { name: cred.name }))) {
                      deleteCredential.mutate(cred.id)
                    }
                  }}
                  className="p-1.5 rounded hover:bg-error-container transition-colors"
                >
                  <span className="material-symbols-outlined text-on-surface-variant text-[18px]">delete</span>
                </button>
              </div>
            </div>
          ))}
        </div>
      ) : activeTab === 'integrations' ? (
        <div className="flex flex-col gap-6">
          {/* Adapters */}
          <div className="bg-surface-container rounded-lg p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="font-headline font-bold text-lg text-on-surface">{t('system_settings.section_adapters')}</h2>
              <button
                onClick={() => setShowCreateAdapter(true)}
                className="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-on-primary-container text-white text-sm font-semibold hover:bg-on-primary-container/90 transition-colors"
              >
                <span className="material-symbols-outlined text-[16px]">add</span>
                {t('system_settings.btn_add_adapter')}
              </button>
            </div>
            <div className="flex flex-col gap-2">
              {adapters.length === 0 && <p className="text-sm text-on-surface-variant">{t('system_settings.no_adapters')}</p>}
              {adapters.map((a: any) => (
                <div key={a.id} className="flex items-center justify-between bg-surface-container-low rounded-lg px-4 py-3">
                  <span className="text-sm text-on-surface font-semibold">{a.name}</span>
                  <span className="text-xs text-on-surface-variant">{a.type} / {a.direction}</span>
                  <span className={`px-2.5 py-1 rounded text-[0.6875rem] font-semibold tracking-wider ${a.enabled ? 'bg-[#064e3b] text-[#34d399]' : 'bg-surface-container-highest text-on-surface-variant'}`}>
                    {a.enabled ? t('system_settings.status_enabled') : t('system_settings.status_disabled')}
                  </span>
                </div>
              ))}
            </div>
          </div>

          {/* Webhooks */}
          <div className="bg-surface-container rounded-lg p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="font-headline font-bold text-lg text-on-surface">{t('system_settings.section_webhooks')}</h2>
              <button
                onClick={() => setShowCreateWebhook(true)}
                className="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-on-primary-container text-white text-sm font-semibold hover:bg-on-primary-container/90 transition-colors"
              >
                <span className="material-symbols-outlined text-[16px]">add</span>
                {t('system_settings.btn_add_webhook')}
              </button>
            </div>
            <div className="flex flex-col gap-2">
              {webhooks.length === 0 && <p className="text-sm text-on-surface-variant">{t('system_settings.no_webhooks')}</p>}
              {webhooks.map((w: any) => (
                <div key={w.id} className="flex items-center justify-between bg-surface-container-low rounded-lg px-4 py-3">
                  <span className="text-sm text-on-surface font-semibold">{w.name}</span>
                  <span className="text-xs text-on-surface-variant truncate max-w-[200px]">{w.url}</span>
                  <span className="text-xs text-on-surface-variant">{w.events?.join(', ')}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      ) : (
      <>
      {/* Stats Row */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        {[
          { icon: 'group', label: t('system_settings.stat_system_users'), value: '1,284', sub: t('system_settings.stat_users_sub'), subColor: 'text-[#34d399]' },
          { icon: 'wifi', label: t('system_settings.stat_active_connections'), value: '12', sub: t('system_settings.stat_connections_sub'), subColor: 'text-on-primary-container' },
          { icon: 'warning', label: t('system_settings.stat_security_level'), value: '3 未解決', sub: t('system_settings.stat_security_sub'), subColor: 'text-[#fbbf24]' },
          { icon: 'shield', label: t('system_settings.stat_system_security'), value: 'High', sub: t('system_settings.stat_security_passed'), subColor: 'text-[#34d399]' },
        ].map((stat, i) => (
          <div key={i} className="bg-surface-container-low rounded-lg p-5 flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <span className="font-label text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant">{stat.label}</span>
              <span className="material-symbols-outlined text-on-surface-variant text-[18px]">{stat.icon}</span>
            </div>
            <div className="font-headline font-bold text-2xl text-on-surface">{stat.value}</div>
            <span className={`text-xs ${stat.subColor}`}>{stat.sub}</span>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* User Management Table */}
        <div className="lg:col-span-2 bg-surface-container rounded-lg p-6">
          <h2 className="font-headline font-bold text-lg text-on-surface mb-4">{t('system_settings.section_user_management')}</h2>

          {/* Table Header */}
          <div className="grid grid-cols-[2fr_1fr_1fr_1fr_1fr_auto] gap-3 px-3 py-2 mb-1">
            <span className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label">{t('system_settings.table_name')}</span>
            <span className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label">{t('system_settings.table_role')}</span>
            <span className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label">{t('system_settings.table_assigned_role')}</span>
            <span className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label">{t('system_settings.table_status')}</span>
            <span className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label">{t('system_settings.table_region')}</span>
            <span className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label">{t('system_settings.table_actions')}</span>
          </div>

          {/* Table Rows */}
          {users.map((user: any, i: number) => (
            <div
              key={user.id ?? i}
              className={`grid grid-cols-[2fr_1fr_1fr_1fr_1fr_auto] gap-3 items-center px-3 py-3 rounded-lg ${
                i % 2 === 0 ? 'bg-surface-container-low' : ''
              }`}
            >
              <div className="flex items-center gap-3">
                <div className="w-8 h-8 rounded-full bg-surface-container-high flex items-center justify-center text-[0.6875rem] font-semibold text-on-surface-variant">
                  {user.avatar}
                </div>
                <span className="text-sm text-on-surface">{user.name}</span>
              </div>
              <span className="text-sm text-on-surface-variant">{user.role}</span>
              {/* Role Assignment */}
              <div>
                {canManageUsers ? (
                  <select
                    value={user.roleId}
                    onChange={(e) => {
                      if (e.target.value && user.id) {
                        assignRole.mutate({ userId: user.id, roleId: e.target.value }, {
                          onSuccess: () => toast.success(t('system_settings.role_assigned'))
                        })
                      }
                    }}
                    className="bg-surface-container-low border border-surface-container-highest rounded px-2 py-1 text-xs text-on-surface"
                  >
                    <option value="">{t('system_settings.select_role')}</option>
                    {apiRoles.map((r: any) => (
                      <option key={r.id} value={r.id}>{r.name}</option>
                    ))}
                  </select>
                ) : (
                  <span className="inline-block px-2 py-0.5 rounded bg-surface-container-high text-[0.6875rem] text-on-surface-variant">
                    {user.role}
                  </span>
                )}
              </div>
              <span
                className={`inline-block w-fit px-2.5 py-1 rounded text-[0.6875rem] font-semibold tracking-wider ${
                  user.status === 'Active'
                    ? 'bg-[#064e3b] text-[#34d399]'
                    : 'bg-surface-container-highest text-on-surface-variant'
                }`}
              >
                {user.status}
              </span>
              <span className="text-sm text-on-surface-variant">{user.region}</span>
              {canManageUsers && (
                <div className="flex gap-1">
                  <button onClick={() => {
                    setEditingUser(user)
                    setNewUserData({ username: user.username ?? '', display_name: user.name ?? '', email: user.email ?? '', password: '', role_id: '' })
                    setShowUserModal(true)
                  }} className="p-1.5 rounded hover:bg-surface-container-high transition-colors">
                    <span className="material-symbols-outlined text-on-surface-variant text-[18px]">edit</span>
                  </button>
                  <button onClick={() => {
                    if (confirm(t('system_settings.confirm_delete_user', { name: user.name }))) {
                      deleteUser.mutate(user.id, { onSuccess: () => toast.success(t('system_settings.user_deleted')) })
                    }
                  }} className="p-1.5 rounded hover:bg-error-container transition-colors">
                    <span className="material-symbols-outlined text-on-surface-variant text-[18px]">delete</span>
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>

        {/* Right Panel - Security Key Health */}
        <div className="flex flex-col gap-6">
          <div className="bg-surface-container rounded-lg p-6">
            <h2 className="font-headline font-bold text-lg text-on-surface mb-1">{t('system_settings.section_security_key_health_zh')}</h2>
            <p className="text-on-surface-variant text-xs mb-4">{t('system_settings.section_security_key_health')}</p>

            <div className="flex flex-col gap-3">
              {healthIndicators.map((item, i) => (
                <div key={i} className="flex items-center justify-between bg-surface-container-low rounded-lg px-4 py-3">
                  <div className="flex items-center gap-3">
                    <span
                      className={`w-2 h-2 rounded-full ${
                        item.status === 'operational' ? 'bg-[#34d399]' : 'bg-[#fbbf24]'
                      }`}
                    />
                    <span className="text-sm text-on-surface">{item.label}</span>
                  </div>
                  <span className="text-xs text-on-surface-variant">{item.latency}</span>
                </div>
              ))}
            </div>
          </div>

          {/* Two-Factor Auth Section */}
          <div className="bg-surface-container rounded-lg p-6">
            <h2 className="font-headline font-bold text-lg text-on-surface mb-1">{t('system_settings.section_2fa_zh')}</h2>
            <p className="text-on-surface-variant text-xs mb-4">{t('system_settings.section_2fa')}</p>

            <div className="flex items-center justify-between bg-surface-container-low rounded-lg px-4 py-3 mb-3">
              <span className="text-sm text-on-surface">{t('system_settings.label_global_2fa_enforcement')}</span>
              <span className="px-2.5 py-1 rounded text-[0.6875rem] font-semibold tracking-wider bg-[#064e3b] text-[#34d399]">
                {t('system_settings.label_2fa_enabled')}
              </span>
            </div>
            <div className="flex items-center justify-between bg-surface-container-low rounded-lg px-4 py-3">
              <span className="text-sm text-on-surface">{t('system_settings.label_compliance_rate')}</span>
              <span className="text-sm font-semibold text-primary">96.2%</span>
            </div>
          </div>
        </div>
      </div>

      {/* QR Authorization Section */}
      <div className="bg-surface-container rounded-lg p-6 mt-6">
        <h2 className="font-headline font-bold text-lg text-on-surface mb-1">{t('system_settings.section_qr_auth_zh')}</h2>
        <p className="text-on-surface-variant text-xs mb-5">{t('system_settings.section_qr_auth')}</p>

        <div className="flex items-center gap-8">
          {/* QR Placeholder */}
          <div className="w-36 h-36 bg-surface-container-low rounded-lg flex items-center justify-center shrink-0">
            <span className="material-symbols-outlined text-on-surface-variant text-[56px]">qr_code_2</span>
          </div>
          <div>
            <p className="text-sm text-on-surface mb-2">{t('system_settings.qr_scan_instruction')}</p>
            <p className="text-xs text-on-surface-variant mb-4">{t('system_settings.qr_validity')}</p>
            <button onClick={() => toast.info('Coming Soon')} className="px-4 py-2 rounded-lg bg-surface-container-high text-on-surface-variant text-sm font-semibold hover:bg-surface-container-highest transition-colors">
              {t('system_settings.btn_regenerate_qr')}
            </button>
          </div>
        </div>
      </div>

      </>
      )}

      {/* System Status Bar */}
      <div className="bg-surface-container-low rounded-lg px-5 py-3 mt-6 flex items-center justify-between">
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <span className="w-2 h-2 rounded-full bg-[#34d399]" />
            <span className="text-xs text-on-surface-variant">{t('system_settings.footer_core_services')}</span>
          </div>
          <div className="flex items-center gap-2">
            <span className="w-2 h-2 rounded-full bg-[#fbbf24]" />
            <span className="text-xs text-on-surface-variant">{t('system_settings.footer_ldap_sync')}</span>
          </div>
        </div>
        <span className="text-xs text-on-surface-variant">{t('system_settings.footer_last_sync', { time: '2 min ago' })}</span>
      </div>

      <CreateAdapterModal open={showCreateAdapter} onClose={() => setShowCreateAdapter(false)} />
      <CreateWebhookModal open={showCreateWebhook} onClose={() => setShowCreateWebhook(false)} />
      <CreateCredentialModal
        open={showCreateCredential}
        onClose={() => { setShowCreateCredential(false); setEditingCredential(null) }}
        editing={editingCredential}
      />

      {showUserModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => { setShowUserModal(false); setEditingUser(null) }}>
          <div className="bg-surface-container p-6 rounded-xl w-96 space-y-4" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-bold text-on-surface">
              {editingUser ? t('system_settings.edit_user_title') : t('system_settings.create_user_title')}
            </h3>
            {!editingUser && (
              <input placeholder={t('system_settings.placeholder_username')} value={newUserData.username}
                onChange={e => setNewUserData(p => ({...p, username: e.target.value}))}
                className="w-full p-2 bg-surface-container-low rounded border border-surface-container-highest text-on-surface" />
            )}
            <input placeholder={t('system_settings.placeholder_display_name')} value={newUserData.display_name}
              onChange={e => setNewUserData(p => ({...p, display_name: e.target.value}))}
              className="w-full p-2 bg-surface-container-low rounded border border-surface-container-highest text-on-surface" />
            <input placeholder={t('system_settings.placeholder_email')} value={newUserData.email}
              onChange={e => setNewUserData(p => ({...p, email: e.target.value}))}
              className="w-full p-2 bg-surface-container-low rounded border border-surface-container-highest text-on-surface" />
            {!editingUser && (
              <input type="password" placeholder={t('system_settings.placeholder_password')} value={newUserData.password}
                onChange={e => setNewUserData(p => ({...p, password: e.target.value}))}
                className="w-full p-2 bg-surface-container-low rounded border border-surface-container-highest text-on-surface" />
            )}
            {!editingUser && (
              <select
                value={newUserData.role_id}
                onChange={e => setNewUserData(p => ({...p, role_id: e.target.value}))}
                className="w-full p-2 bg-surface-container-low rounded border border-surface-container-highest text-on-surface"
                required
              >
                <option value="">{t('system_settings.select_role')}</option>
                {apiRoles.map((r: any) => (
                  <option key={r.id} value={r.id}>{r.name}</option>
                ))}
              </select>
            )}
            <div className="flex gap-2 justify-end">
              <button onClick={() => { setShowUserModal(false); setEditingUser(null) }} className="px-4 py-2 rounded bg-surface-container-high text-on-surface-variant">{t('system_settings.btn_cancel')}</button>
              {editingUser ? (
                <button onClick={() => updateUser.mutate({ id: editingUser.id, data: { display_name: newUserData.display_name, email: newUserData.email } }, {
                  onSuccess: () => { setShowUserModal(false); setEditingUser(null); toast.success(t('system_settings.user_updated')) }
                })}
                  disabled={updateUser.isPending} className="px-4 py-2 rounded bg-on-primary-container text-white disabled:opacity-50">
                  {updateUser.isPending ? t('common.saving') : t('common.save_changes')}
                </button>
              ) : (
                <button onClick={() => {
                  if (!newUserData.role_id) { toast.error(t('system_settings.role_required')); return }
                  const { role_id, ...userData } = newUserData
                  createUser.mutate(userData, {
                    onSuccess: (resp: any) => {
                      const userId = resp?.data?.id
                      if (userId && role_id) {
                        assignRole.mutate({ userId, roleId: role_id })
                      }
                      setShowUserModal(false)
                      setNewUserData({ username: '', display_name: '', email: '', password: '', role_id: '' })
                      toast.success(t('system_settings.user_created'))
                    }
                  })
                }}
                  disabled={createUser.isPending} className="px-4 py-2 rounded bg-on-primary-container text-white disabled:opacity-50">
                  {createUser.isPending ? t('system_settings.btn_creating') : t('system_settings.btn_create')}
                </button>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
