import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useCreateAdapter } from '../hooks/useIntegration'

interface Props {
  open: boolean
  onClose: () => void
}

const initial = {
  name: '',
  type: 'prometheus',
  direction: 'inbound',
  endpoint: '',
  enabled: true,
  // prometheus
  queries: '',
  pull_interval: '300',
  // zabbix
  zabbix_api_token: '',
  zabbix_username: '',
  zabbix_password: '',
  zabbix_host_groups: '',
  zabbix_items: '',
  // custom_rest
  custom_url: '',
  custom_headers: '',
  custom_method: 'GET',
  custom_body: '',
  custom_result_path: '',
  custom_name_field: '',
  custom_value_field: '',
  custom_timestamp_field: '',
  custom_ip_field: '',
}

const COMING_SOON_TYPES = ['snmp', 'datadog', 'nagios']

export default function CreateAdapterModal({ open, onClose }: Props) {
  const { t } = useTranslation()
  const [formData, setFormData] = useState({ ...initial })
  const mutation = useCreateAdapter()

  if (!open) return null

  const isPrometheus = formData.type === 'prometheus'
  const isZabbix = formData.type === 'zabbix'
  const isCustomREST = formData.type === 'custom_rest'
  const isComingSoon = COMING_SOON_TYPES.includes(formData.type)

  const buildConfig = (): Record<string, unknown> | undefined => {
    if (isPrometheus && formData.queries.trim()) {
      return {
        queries: formData.queries.split('\n').map(q => q.trim()).filter(Boolean),
        pull_interval_seconds: parseInt(formData.pull_interval, 10),
      }
    }
    if (isZabbix) {
      const cfg: Record<string, unknown> = {
        host_groups: formData.zabbix_host_groups.split(',').map(s => s.trim()).filter(Boolean),
        items: formData.zabbix_items.split('\n').map(s => s.trim()).filter(Boolean),
      }
      if (formData.zabbix_api_token.trim()) {
        cfg.api_token = formData.zabbix_api_token.trim()
      } else if (formData.zabbix_username.trim()) {
        cfg.username = formData.zabbix_username.trim()
        cfg.password = formData.zabbix_password
      }
      return cfg
    }
    if (isCustomREST) {
      const headers: Record<string, string> = {}
      formData.custom_headers.split('\n').forEach(line => {
        const idx = line.indexOf(':')
        if (idx > 0) {
          headers[line.slice(0, idx).trim()] = line.slice(idx + 1).trim()
        }
      })
      return {
        url: formData.custom_url || undefined,
        headers,
        method: formData.custom_method,
        body: formData.custom_body || undefined,
        result_path: formData.custom_result_path || undefined,
        name_field: formData.custom_name_field || undefined,
        value_field: formData.custom_value_field || undefined,
        timestamp_field: formData.custom_timestamp_field || undefined,
        ip_field: formData.custom_ip_field || undefined,
      }
    }
    return undefined
  }

  const handleCreate = () => {
    const config = buildConfig()
    const payload: Record<string, unknown> = {
      name: formData.name,
      type: formData.type,
      direction: formData.direction,
      endpoint: formData.endpoint,
      enabled: formData.enabled,
    }
    if (config !== undefined) {
      payload.config = config
    }
    mutation.mutate(payload as any, {
      onSuccess: () => { onClose(); setFormData({ ...initial }) },
    })
  }

  const inputCls = 'w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm'
  const labelCls = 'block text-sm text-gray-400 mb-1'

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-[#1a1f2e] p-6 rounded-xl w-[28rem] space-y-4 max-h-[90vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
        <h3 className="text-lg font-bold text-white">{t('adapter_modal.title')}</h3>

        <div>
          <label className={labelCls}>{t('adapter_modal.name_label')} *</label>
          <input value={formData.name} onChange={e => setFormData(p => ({ ...p, name: e.target.value }))}
            className={inputCls} placeholder={t('adapter_modal.name_placeholder')} />
        </div>

        <div>
          <label className={labelCls}>{t('adapter_modal.type_label')}</label>
          <select value={formData.type} onChange={e => setFormData(p => ({ ...p, type: e.target.value }))}
            className={inputCls}>
            <option value="prometheus">{t('adapter_types.prometheus')}</option>
            <option value="zabbix">{t('adapter_types.zabbix')}</option>
            <option value="custom_rest">{t('adapter_types.custom_rest')}</option>
            <option value="snmp">{t('adapter_types.snmp')}</option>
            <option value="datadog">{t('adapter_types.datadog')}</option>
            <option value="nagios">{t('adapter_types.nagios')}</option>
            <option value="dify">{t('adapter_types.dify')}</option>
          </select>
        </div>

        <div>
          <label className={labelCls}>{t('adapter_modal.direction_label')}</label>
          <select value={formData.direction} onChange={e => setFormData(p => ({ ...p, direction: e.target.value }))}
            className={inputCls}>
            <option value="inbound">{t('adapter_modal.direction_inbound')}</option>
            <option value="outbound">{t('adapter_modal.direction_outbound')}</option>
            <option value="bidirectional">{t('adapter_modal.direction_bidirectional')}</option>
          </select>
        </div>

        <div>
          <label className={labelCls}>{t('adapter_modal.endpoint_label')}</label>
          <input value={formData.endpoint} onChange={e => setFormData(p => ({ ...p, endpoint: e.target.value }))}
            className={inputCls} placeholder="https://..." />
        </div>

        {isComingSoon && (
          <div className="p-3 rounded bg-yellow-900/30 border border-yellow-700 text-yellow-300 text-sm">
            {t('adapter_config.coming_soon')}
          </div>
        )}

        {isPrometheus && (
          <>
            <div>
              <label className={labelCls}>Metric Queries (one per line)</label>
              <textarea
                value={formData.queries}
                onChange={e => setFormData(p => ({ ...p, queries: e.target.value }))}
                rows={4}
                className={`${inputCls} font-mono`}
                placeholder={"node_cpu_seconds_total\nnode_memory_MemAvailable_bytes\npower_kw"}
              />
            </div>
            <div>
              <label className={labelCls}>Pull Interval</label>
              <select value={formData.pull_interval} onChange={e => setFormData(p => ({ ...p, pull_interval: e.target.value }))}
                className={inputCls}>
                <option value="60">1 minute</option>
                <option value="300">5 minutes (default)</option>
                <option value="900">15 minutes</option>
                <option value="1800">30 minutes</option>
                <option value="3600">1 hour</option>
              </select>
            </div>
          </>
        )}

        {isZabbix && (
          <>
            <div>
              <label className={labelCls}>{t('adapter_config.api_token')}</label>
              <input value={formData.zabbix_api_token}
                onChange={e => setFormData(p => ({ ...p, zabbix_api_token: e.target.value }))}
                className={inputCls} placeholder="zabbix_api_token (or use username/password below)" />
            </div>
            <div className="text-xs text-gray-500 text-center">— or —</div>
            <div>
              <label className={labelCls}>{t('adapter_config.username')}</label>
              <input value={formData.zabbix_username}
                onChange={e => setFormData(p => ({ ...p, zabbix_username: e.target.value }))}
                className={inputCls} />
            </div>
            <div>
              <label className={labelCls}>{t('adapter_config.password')}</label>
              <input type="password" value={formData.zabbix_password}
                onChange={e => setFormData(p => ({ ...p, zabbix_password: e.target.value }))}
                className={inputCls} />
            </div>
            <div>
              <label className={labelCls}>{t('adapter_config.host_groups')}</label>
              <input value={formData.zabbix_host_groups}
                onChange={e => setFormData(p => ({ ...p, zabbix_host_groups: e.target.value }))}
                className={inputCls} placeholder="Linux servers, Windows hosts" />
            </div>
            <div>
              <label className={labelCls}>{t('adapter_config.zabbix_items')}</label>
              <textarea
                value={formData.zabbix_items}
                onChange={e => setFormData(p => ({ ...p, zabbix_items: e.target.value }))}
                rows={4}
                className={`${inputCls} font-mono`}
                placeholder={t('adapter_config.zabbix_items_placeholder')}
              />
            </div>
          </>
        )}

        {isCustomREST && (
          <>
            <div>
              <label className={labelCls}>{t('adapter_config.custom_url')}</label>
              <input value={formData.custom_url}
                onChange={e => setFormData(p => ({ ...p, custom_url: e.target.value }))}
                className={inputCls} placeholder="https://api.example.com/metrics" />
            </div>
            <div>
              <label className={labelCls}>{t('adapter_config.custom_headers')}</label>
              <textarea
                value={formData.custom_headers}
                onChange={e => setFormData(p => ({ ...p, custom_headers: e.target.value }))}
                rows={3}
                className={`${inputCls} font-mono`}
                placeholder={"Authorization: Bearer TOKEN\nX-API-Key: mykey"}
              />
            </div>
            <div>
              <label className={labelCls}>{t('adapter_config.custom_method')}</label>
              <select value={formData.custom_method}
                onChange={e => setFormData(p => ({ ...p, custom_method: e.target.value }))}
                className={inputCls}>
                <option value="GET">GET</option>
                <option value="POST">POST</option>
              </select>
            </div>
            {formData.custom_method === 'POST' && (
              <div>
                <label className={labelCls}>{t('adapter_config.custom_body')}</label>
                <textarea
                  value={formData.custom_body}
                  onChange={e => setFormData(p => ({ ...p, custom_body: e.target.value }))}
                  rows={3}
                  className={`${inputCls} font-mono`}
                  placeholder='{"query": "..."}'
                />
              </div>
            )}
            <div>
              <label className={labelCls}>{t('adapter_config.result_path')}</label>
              <input value={formData.custom_result_path}
                onChange={e => setFormData(p => ({ ...p, custom_result_path: e.target.value }))}
                className={inputCls} placeholder={t('adapter_config.result_path_placeholder')} />
            </div>
            <div className="grid grid-cols-2 gap-2">
              <div>
                <label className={labelCls}>{t('adapter_config.name_field')}</label>
                <input value={formData.custom_name_field}
                  onChange={e => setFormData(p => ({ ...p, custom_name_field: e.target.value }))}
                  className={inputCls} placeholder="name" />
              </div>
              <div>
                <label className={labelCls}>{t('adapter_config.value_field')}</label>
                <input value={formData.custom_value_field}
                  onChange={e => setFormData(p => ({ ...p, custom_value_field: e.target.value }))}
                  className={inputCls} placeholder="value" />
              </div>
              <div>
                <label className={labelCls}>{t('adapter_config.timestamp_field')}</label>
                <input value={formData.custom_timestamp_field}
                  onChange={e => setFormData(p => ({ ...p, custom_timestamp_field: e.target.value }))}
                  className={inputCls} placeholder="timestamp" />
              </div>
              <div>
                <label className={labelCls}>{t('adapter_config.ip_field')}</label>
                <input value={formData.custom_ip_field}
                  onChange={e => setFormData(p => ({ ...p, custom_ip_field: e.target.value }))}
                  className={inputCls} placeholder="ip" />
              </div>
            </div>
          </>
        )}

        <div className="flex items-center gap-2">
          <input type="checkbox" checked={formData.enabled} onChange={e => setFormData(p => ({ ...p, enabled: e.target.checked }))}
            className="rounded border-gray-700" />
          <label className="text-sm text-gray-400">{t('adapter_modal.enabled_label')}</label>
        </div>

        <div className="flex gap-2 justify-end pt-2">
          <button onClick={onClose} className="px-4 py-2 rounded bg-gray-700 text-white text-sm">{t('adapter_modal.btn_cancel')}</button>
          <button onClick={handleCreate} disabled={mutation.isPending || !formData.name || isComingSoon}
            className="px-4 py-2 rounded bg-blue-600 text-white text-sm disabled:opacity-50">
            {mutation.isPending ? t('adapter_modal.btn_creating') : t('adapter_modal.btn_create')}
          </button>
        </div>
      </div>
    </div>
  )
}
