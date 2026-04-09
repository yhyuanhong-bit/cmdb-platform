import { toast } from 'sonner'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../stores/authStore'
import { useUpdateUser, useUserSessions, useChangePassword } from '../hooks/useIdentity'
import { useSystemHealth } from '../hooks/useSystemHealth'

export default function UserProfile() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const user = useAuthStore((s) => s.user)
  const updateUser = useUpdateUser()

  const { data: healthResp } = useSystemHealth()
  const dbLatency = (healthResp as any)?.data?.database?.latency_ms
  const connectivity = dbLatency ? (dbLatency < 50 ? 'Good' : dbLatency < 200 ? 'Normal' : 'Degraded') : '—'

  const { data: sessionsData } = useUserSessions(user?.id || '')
  const sessions = (sessionsData as any)?.sessions ?? []
  const changePasswordMutation = useChangePassword()

  const [showPwModal, setShowPwModal] = useState(false)
  const [currentPw, setCurrentPw] = useState('')
  const [newPw, setNewPw] = useState('')

  const handlePasswordChange = () => {
    changePasswordMutation.mutate({ current_password: currentPw, new_password: newPw }, {
      onSuccess: () => { setShowPwModal(false); setCurrentPw(''); setNewPw(''); toast.success('Password changed successfully!') },
      onError: () => toast.error('Failed to change password. Check your current password.')
    })
  }

  const [displayName, setDisplayName] = useState(user?.display_name ?? 'OPERATOR_042')
  const [employeeId, setEmployeeId] = useState(user?.id?.substring(0, 12) ?? 'IG-992-042')
  const [department, setDepartment] = useState('core_infra')
  const [emailAlerts, setEmailAlerts] = useState(true)
  const [slackIntegration, setSlackIntegration] = useState(true)
  const [pushNotifications, setPushNotifications] = useState(false)

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
          系統
        </span>
        <span className="text-[10px] opacity-40" aria-hidden="true">›</span>
        <span className="text-on-surface font-semibold">{t('user_profile.role_badge')}</span>
      </nav>

      {/* Header */}
      <div className="bg-surface-container rounded-lg p-6 mb-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-5">
            <div className="w-20 h-20 rounded-full bg-surface-container-high flex items-center justify-center">
              <span className="material-symbols-outlined text-on-surface-variant text-[40px]">person</span>
            </div>
            <div>
              <h1 className="font-headline font-bold text-2xl text-on-surface">{user?.display_name ?? user?.username ?? 'OPERATOR_042'}</h1>
              <p className="text-on-surface-variant text-sm mt-1">{user?.email || '—'}</p>
              <span className="inline-block mt-2 px-2.5 py-1 rounded text-[0.6875rem] font-semibold uppercase tracking-wider bg-on-primary-container/20 text-on-primary-container">
                {t('user_profile.role_badge')}
              </span>
            </div>
          </div>
          <div className="text-right">
            <span className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant">{t('user_profile.label_last_login_ip')}</span>
            <p className="font-headline font-semibold text-on-surface mt-1">—</p>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Account Settings */}
        <div className="bg-surface-container rounded-lg p-6">
          <h2 className="font-headline font-bold text-lg text-on-surface mb-1">{t('user_profile.section_account_settings_zh')}</h2>
          <p className="text-on-surface-variant text-xs mb-5">{t('user_profile.section_account_settings')}</p>

          <div className="flex flex-col gap-4">
            <div>
              <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
                {t('user_profile.label_display_name')}
              </label>
              <input
                type="text"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                className="w-full bg-surface-container-low rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-on-surface-variant/50 outline-none focus:ring-1 focus:ring-primary/40"
              />
            </div>
            <div>
              <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
                {t('user_profile.label_employee_id')}
              </label>
              <input
                type="text"
                value={employeeId}
                onChange={(e) => setEmployeeId(e.target.value)}
                className="w-full bg-surface-container-low rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-on-surface-variant/50 outline-none focus:ring-1 focus:ring-primary/40"
              />
            </div>
            <div>
              <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
                {t('user_profile.label_department')}
              </label>
              <select
                value={department}
                onChange={(e) => setDepartment(e.target.value)}
                className="w-full bg-surface-container-low rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40 appearance-none"
              >
                <option value="core_infra">{t('user_profile.dept_core_infra')}</option>
                <option value="network_ops">{t('user_profile.dept_network_ops')}</option>
                <option value="security">{t('user_profile.dept_security')}</option>
              </select>
            </div>
            <button
              onClick={() => { const u = useAuthStore.getState().user; if (u?.id) updateUser.mutate({ id: u.id, data: { display_name: displayName } }) }}
              disabled={updateUser.isPending}
              className="self-start mt-2 px-5 py-2.5 rounded-lg bg-on-primary-container text-white text-sm font-semibold hover:bg-on-primary-container/90 transition-colors disabled:opacity-50"
            >
              {updateUser.isPending ? 'Saving...' : t('user_profile.btn_update_profile')}
            </button>
          </div>
        </div>

        {/* Security */}
        <div className="bg-surface-container rounded-lg p-6">
          <h2 className="font-headline font-bold text-lg text-on-surface mb-1">{t('user_profile.section_security_zh')}</h2>
          <p className="text-on-surface-variant text-xs mb-5">{t('user_profile.section_security')}</p>

          {/* Change Password */}
          <button onClick={() => setShowPwModal(true)} className="w-full flex items-center justify-between bg-surface-container-low rounded-lg px-4 py-3 mb-3 hover:bg-surface-container-high transition-colors">
            <div className="flex items-center gap-3">
              <span className="material-symbols-outlined text-on-surface-variant text-[20px]">lock</span>
              <span className="text-sm text-on-surface">{t('user_profile.btn_change_password')}</span>
            </div>
            <span className="material-symbols-outlined text-on-surface-variant text-[18px]">chevron_right</span>
          </button>

          {/* 2FA */}
          <div className="bg-surface-container-low rounded-lg px-4 py-4 mb-3">
            <div className="flex items-center justify-between mb-2">
              <div className="flex items-center gap-3">
                <span className="material-symbols-outlined text-on-surface-variant text-[20px]">security</span>
                <span className="text-sm text-on-surface">{t('user_profile.label_2fa')}</span>
              </div>
              <span className="px-2.5 py-1 rounded text-[0.6875rem] font-semibold tracking-wider bg-[#064e3b] text-[#34d399]">
                {t('user_profile.label_2fa_enabled')}
              </span>
            </div>
            <p className="text-on-surface-variant text-xs ml-8 mb-3">{t('user_profile.label_2fa_provider')}</p>
            <button onClick={() => toast.info('Coming Soon')} className="ml-8 px-4 py-2 rounded-lg bg-surface-container-high text-on-surface-variant text-xs font-semibold hover:bg-surface-container-highest transition-colors">
              {t('user_profile.btn_reset_2fa_key')}
            </button>
          </div>

          {/* Active Sessions */}
          <div className="bg-surface-container-low rounded-lg px-4 py-4">
            <h3 className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label mb-3">
              {t('user_profile.section_active_sessions')}
            </h3>
            <div className="flex flex-col gap-2">
              {sessions.map((s: any, i: number) => (
                <div key={i} className="flex items-center justify-between bg-surface-container rounded-lg px-3 py-2.5">
                  <div className="flex items-center gap-3">
                    <span className="material-symbols-outlined text-on-surface-variant text-[18px]">{s.icon}</span>
                    <div>
                      <p className="text-sm text-on-surface">{s.device}</p>
                      <p className="text-xs text-on-surface-variant">{s.time}</p>
                    </div>
                  </div>
                  {!s.current && (
                    <button onClick={() => toast.info('Coming Soon')} className="px-3 py-1.5 rounded text-[0.6875rem] font-semibold bg-error-container text-on-error-container hover:bg-error-container/80 transition-colors">
                      {t('user_profile.btn_revoke')}
                    </button>
                  )}
                  {s.current && (
                    <span className="text-[0.6875rem] text-on-surface-variant">{t('user_profile.label_current_device')}</span>
                  )}
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Notification Preferences */}
        <div className="bg-surface-container rounded-lg p-6">
          <h2 className="font-headline font-bold text-lg text-on-surface mb-1">{t('user_profile.section_notifications_zh')}</h2>
          <p className="text-on-surface-variant text-xs mb-5">{t('user_profile.section_notifications')}</p>

          <div className="flex flex-col gap-3">
            {[
              { label: t('user_profile.notification_email_zh'), sub: t('user_profile.notification_email'), value: emailAlerts, setter: setEmailAlerts },
              { label: t('user_profile.notification_slack_zh'), sub: t('user_profile.notification_slack'), value: slackIntegration, setter: setSlackIntegration },
              { label: t('user_profile.notification_push_zh'), sub: t('user_profile.notification_push'), value: pushNotifications, setter: setPushNotifications },
            ].map((item, i) => (
              <div key={i} className="flex items-center justify-between bg-surface-container-low rounded-lg px-4 py-3">
                <div>
                  <p className="text-sm text-on-surface">{item.label}</p>
                  <p className="text-xs text-on-surface-variant">{item.sub}</p>
                </div>
                <button
                  onClick={() => item.setter(!item.value)}
                  className={`relative w-11 h-6 rounded-full transition-colors ${
                    item.value ? 'bg-on-primary-container' : 'bg-surface-container-highest'
                  }`}
                >
                  <span
                    className={`absolute top-1 left-1 w-4 h-4 rounded-full bg-white transition-transform ${
                      item.value ? 'translate-x-5' : 'translate-x-0'
                    }`}
                  />
                </button>
              </div>
            ))}
          </div>
        </div>

        {/* System Connectivity */}
        <div className="bg-surface-container rounded-lg p-6">
          <h2 className="font-headline font-bold text-lg text-on-surface mb-1">{t('user_profile.section_system_connectivity')}</h2>
          <p className="text-on-surface-variant text-xs mb-5">{t('user_profile.label_api_responsiveness')}</p>

          <div className="flex items-end gap-4">
            <span className="font-headline font-bold text-4xl text-primary">{connectivity}</span>
            <span className="text-on-surface-variant text-sm mb-1">{t('user_profile.label_api_responsiveness')}</span>
          </div>
          <div className="mt-4 w-full bg-surface-container-low rounded-full h-2">
            <div className="bg-on-primary-container h-2 rounded-full" style={{ width: connectivity === 'Good' ? '100%' : connectivity === 'Normal' ? '70%' : '40%' }} />
          </div>
          <div className="flex items-center gap-2 mt-3">
            <span className="material-symbols-outlined text-[#34d399] text-[16px]">check_circle</span>
            <span className="text-xs text-on-surface-variant">{t('user_profile.all_core_services_connected')}</span>
          </div>
        </div>
      </div>

      {/* Password Change Modal */}
      {showPwModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="bg-surface-container rounded-lg p-6 w-full max-w-md shadow-xl">
            <h2 className="font-headline font-bold text-lg text-on-surface mb-4">{t('user_profile.btn_change_password')}</h2>
            <div className="flex flex-col gap-4">
              <div>
                <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
                  Current Password
                </label>
                <input
                  type="password"
                  value={currentPw}
                  onChange={(e) => setCurrentPw(e.target.value)}
                  className="w-full bg-surface-container-low rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40"
                />
              </div>
              <div>
                <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
                  New Password
                </label>
                <input
                  type="password"
                  value={newPw}
                  onChange={(e) => setNewPw(e.target.value)}
                  className="w-full bg-surface-container-low rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40"
                />
              </div>
              <div className="flex gap-3 justify-end mt-2">
                <button
                  onClick={() => { setShowPwModal(false); setCurrentPw(''); setNewPw('') }}
                  className="px-4 py-2 rounded-lg bg-surface-container-high text-sm text-on-surface-variant hover:text-on-surface transition-colors"
                >
                  Cancel
                </button>
                <button
                  onClick={handlePasswordChange}
                  disabled={!currentPw || !newPw || changePasswordMutation.isPending}
                  className="px-4 py-2 rounded-lg bg-on-primary-container text-white text-sm font-semibold hover:bg-on-primary-container/90 transition-colors disabled:opacity-50"
                >
                  {changePasswordMutation.isPending ? 'Saving...' : 'Change Password'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
