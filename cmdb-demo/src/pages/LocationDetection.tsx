import { toast } from 'sonner'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { apiClient } from '../lib/api/client'

export default function LocationDetection() {
  const { t } = useTranslation()

  const { data: summary, refetch } = useQuery({
    queryKey: ['locationDetectSummary'],
    queryFn: () => apiClient.get('/location-detect/summary'),
    refetchInterval: 60000,
  })
  const s = summary as Record<string, unknown> | undefined

  const { data: diffsData, isLoading: diffsLoading, refetch: refetchDiffs } = useQuery({
    queryKey: ['locationDetectDiffs'],
    queryFn: () => apiClient.get('/location-detect/diffs'),
    refetchInterval: 5 * 60 * 1000,
  })
  const diffs = ((diffsData as Record<string, unknown>)?.data as Array<Record<string, unknown>> ?? []).filter((d: Record<string, unknown>) => d.diff_type !== 'consistent')

  const [scanning, setScanning] = useState(false)

  const handleScan = async () => {
    setScanning(true)
    try {
      const result = await apiClient.post('/ingestion/mac-scan', {}) as { scanned_ips?: number; entries_collected?: number }
      const ips = result?.scanned_ips ?? 0
      const entries = result?.entries_collected ?? 0
      if (ips === 0) {
        toast.info(t('location_detect.no_targets', 'No scan targets found. Add switch management IPs in Asset Management or configure scan targets.'))
      } else {
        toast.success(t('location_detect.scan_complete', { ips, entries, defaultValue: `Scan complete: ${ips} switches scanned, ${entries} MAC entries collected` }))
      }
      setTimeout(() => { refetch(); refetchDiffs() }, 3000)
    } catch {
      toast.error(t('location_detect.scan_failed', 'Scan failed — check if ingestion-engine is running and SNMP credentials are configured'))
    } finally {
      setScanning(false)
    }
  }

  const trackedByNetwork = (s?.tracked_by_network as number) ?? 0
  const coveragePct = s?.coverage_pct as number | undefined
  const relocations24h = (s?.relocations_24h as number) ?? 0
  const unregistered = (s?.unregistered as number) ?? 0

  const stats = [
    { label: t('location_detect.tracked', 'Tracked Devices'), value: trackedByNetwork || '\u2014', icon: 'wifi', color: 'text-[#69db7c]' },
    { label: t('location_detect.coverage', 'Coverage'), value: coveragePct ? `${Math.round(coveragePct)}%` : '\u2014', icon: 'pie_chart', color: 'text-primary' },
    { label: t('location_detect.relocations', 'Relocations (24h)'), value: relocations24h, icon: 'swap_horiz', color: 'text-tertiary' },
    { label: t('location_detect.unregistered', 'Unregistered'), value: unregistered, icon: 'device_unknown', color: 'text-error' },
  ]

  return (
    <div className="min-h-screen bg-surface p-6 font-body text-on-surface">
      {/* Header */}
      <div className="mb-6">
        <h1 className="font-headline text-2xl font-bold tracking-tight text-on-surface">
          {t('location_detect.page_title', 'Location Detection')}
        </h1>
        <p className="text-sm text-on-surface-variant mt-1">
          {t('location_detect.page_subtitle', 'Automatic asset location tracking via SNMP/CDP network topology')}
        </p>
      </div>

      <div className="space-y-6">
        {/* Status Card */}
        <div className="bg-surface-container rounded-xl p-6">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-3">
              <span className="material-symbols-outlined text-primary text-2xl">radar</span>
              <div>
                <h3 className="font-headline font-bold text-on-surface">
                  {t('location_detect.title', 'SNMP Location Detection')}
                </h3>
                <p className="text-xs text-on-surface-variant">
                  {t('location_detect.subtitle', 'Automatic asset location tracking via CDP/MAC table analysis')}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <span className="text-xs text-on-surface-variant">
                {t('location_detect.last_scan', 'Last scan')}: {trackedByNetwork > 0 ? t('location_detect.active', 'Active') : t('location_detect.no_data', 'No data yet')}
              </span>
              <div className="w-2 h-2 rounded-full bg-[#69db7c] animate-ping" />
            </div>
          </div>

          <div className="grid grid-cols-4 gap-4">
            {stats.map(st => (
              <div key={st.label} className="bg-surface-container-low rounded-xl p-4">
                <div className="flex items-center gap-2 mb-2">
                  <span className={`material-symbols-outlined text-lg ${st.color}`}>{st.icon}</span>
                  <span className="text-[10px] text-on-surface-variant font-label uppercase tracking-widest">{st.label}</span>
                </div>
                <div className="text-2xl font-headline font-bold text-on-surface">{st.value}</div>
              </div>
            ))}
          </div>
        </div>

        {/* Actions */}
        <div className="flex items-center gap-3">
          <button
            onClick={handleScan}
            disabled={scanning}
            className="bg-primary hover:opacity-90 text-on-primary px-4 py-2 rounded-xl text-sm font-label font-bold flex items-center gap-2 transition-opacity disabled:opacity-50"
          >
            <span className="material-symbols-outlined text-lg">{scanning ? 'hourglass_top' : 'radar'}</span>
            {scanning ? t('location_detect.scanning', 'Scanning...') : t('location_detect.scan_now', 'Scan Now')}
          </button>
          <button
            onClick={() => window.open('/api/v1/location-detect/report?days=30', '_blank')}
            className="bg-surface-container-high hover:bg-surface-container-highest text-on-surface-variant px-4 py-2 rounded-xl text-sm font-label font-bold flex items-center gap-2 transition-colors"
          >
            <span className="material-symbols-outlined text-lg">summarize</span>
            {t('location_detect.download_report', 'Download Monthly Report')}
          </button>
        </div>

        {/* Setup Guide */}
        <div className="bg-surface-container rounded-xl p-6">
          <h4 className="font-headline font-bold text-on-surface mb-3">
            {t('location_detect.setup_title', 'Setup Guide')}
          </h4>
          <div className="space-y-3">
            {[
              { step: '1', text: t('location_detect.setup_1', 'Register switches in Asset Management (type=network) with correct rack location'), icon: 'dns' },
              { step: '2', text: t('location_detect.setup_2', 'Configure SNMP credentials in Integrations tab'), icon: 'key' },
              { step: '3', text: t('location_detect.setup_3', 'Set scan target CIDR range for switch management IPs'), icon: 'lan' },
              { step: '4', text: t('location_detect.setup_4', 'Click "Scan Now" to verify -- detection runs automatically every 5 minutes'), icon: 'check_circle' },
            ].map(item => (
              <div key={item.step} className="flex items-start gap-3">
                <div className="w-6 h-6 rounded-full bg-primary-container flex items-center justify-center shrink-0 mt-0.5">
                  <span className="text-[10px] font-bold text-primary">{item.step}</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="material-symbols-outlined text-on-surface-variant text-lg">{item.icon}</span>
                  <span className="text-sm text-on-surface-variant">{item.text}</span>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Detection Results */}
        <div className="bg-surface-container rounded-xl p-6">
          <div className="flex items-center justify-between mb-4">
            <h4 className="font-headline font-bold text-on-surface flex items-center gap-2">
              <span className="material-symbols-outlined text-lg">compare_arrows</span>
              {t('location_detect.results_title', 'Detection Results')}
            </h4>
            <button onClick={() => refetchDiffs()} className="text-xs text-primary hover:underline">
              {t('common.refresh', 'Refresh')}
            </button>
          </div>

          {diffsLoading ? (
            <div className="flex justify-center py-8">
              <div className="animate-spin rounded-full h-6 w-6 border-2 border-primary border-t-transparent" />
            </div>
          ) : diffs.length === 0 ? (
            <div className="text-center py-8 text-on-surface-variant text-sm">
              {t('location_detect.no_diffs', 'All asset locations are consistent')}
            </div>
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-[10px] text-on-surface-variant uppercase tracking-widest border-b border-surface-container-high">
                  <th className="px-3 py-2">{t('location_detect.col_asset', 'Asset')}</th>
                  <th className="px-3 py-2">{t('location_detect.col_type', 'Type')}</th>
                  <th className="px-3 py-2">{t('location_detect.col_cmdb_location', 'CMDB Location')}</th>
                  <th className="px-3 py-2">{t('location_detect.col_actual_location', 'Detected Location')}</th>
                  <th className="px-3 py-2">{t('location_detect.col_action', 'Action')}</th>
                </tr>
              </thead>
              <tbody>
                {diffs.map((d: Record<string, unknown>, i: number) => (
                  <tr key={(d.asset_id as string) || i} className="border-b border-surface-container-high/50 hover:bg-surface-container-low transition-colors">
                    <td className="px-3 py-2.5">
                      <span className="font-medium text-on-surface">{(d.asset_tag as string) || (d.mac_address as string) || '\u2014'}</span>
                    </td>
                    <td className="px-3 py-2.5">
                      <span className={`text-[10px] font-bold px-2 py-0.5 rounded ${
                        d.diff_type === 'relocated' ? 'bg-tertiary-container text-tertiary' :
                        d.diff_type === 'missing' ? 'bg-error-container text-error' :
                        d.diff_type === 'new_device' ? 'bg-primary-container text-primary' :
                        'bg-surface-container-high text-on-surface-variant'
                      }`}>
                        {d.diff_type === 'relocated' ? t('location_detect.relocated', 'RELOCATED') :
                         d.diff_type === 'missing' ? t('location_detect.missing', 'MISSING') :
                         d.diff_type === 'new_device' ? t('location_detect.new_device', 'NEW DEVICE') :
                         t('location_detect.consistent', 'OK')}
                      </span>
                    </td>
                    <td className="px-3 py-2.5 text-on-surface-variant">{(d.cmdb_rack_name as string) || '\u2014'}</td>
                    <td className="px-3 py-2.5 text-on-surface-variant">{(d.actual_rack_name as string) || '\u2014'}</td>
                    <td className="px-3 py-2.5">
                      {d.diff_type === 'relocated' && !d.has_work_order && (
                        <button
                          onClick={() => {
                            if (d.asset_id && d.actual_rack_id) {
                              apiClient.post(`/assets/${d.asset_id}/confirm-location`, { rack_id: d.actual_rack_id })
                                .then(() => { toast.success(t('location_detect.location_confirmed', 'Location confirmed')); refetchDiffs(); })
                                .catch(() => toast.error(t('location_detect.confirm_failed', 'Failed to confirm')))
                            }
                          }}
                          className="text-[10px] font-bold px-2.5 py-1 rounded-lg bg-primary-container text-primary hover:opacity-90 transition-opacity"
                        >
                          {t('location_detect.confirm', 'Confirm')}
                        </button>
                      )}
                      {d.diff_type === 'relocated' && Boolean(d.has_work_order) && (
                        <span className="text-[10px] text-[#69db7c]">{t('location_detect.auto_confirmed', 'Auto-confirmed')}</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  )
}
