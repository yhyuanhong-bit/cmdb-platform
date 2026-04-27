import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import Icon from '../components/Icon'
import { useEnergyBill, useAggregateEnergyRange } from '../hooks/useEnergyBilling'
import { useAssets } from '../hooks/useAssets'

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

const todayISO = () => new Date().toISOString().slice(0, 10)
const daysAgoISO = (n: number) => {
  const d = new Date()
  d.setDate(d.getDate() - n)
  return d.toISOString().slice(0, 10)
}

// Format a decimal string with a sensible number of fraction digits for
// display. Backend returns values as strings to avoid float drift; we keep
// them as strings for arithmetic too and only format for the user.
function formatDecimal(s: string, maxFractionDigits = 4): string {
  const n = Number(s)
  if (!isFinite(n)) return s
  // For integer-valued totals show no decimals; for fractional, up to 4.
  if (n === Math.round(n)) return n.toString()
  const trimmed = n.toFixed(maxFractionDigits)
  return trimmed.replace(/0+$/, '').replace(/\.$/, '')
}

function formatCurrency(amount: string, currency: string): string {
  const n = Number(amount)
  if (!isFinite(n)) return `${amount} ${currency}`
  // 2 decimals for major currencies, 4 for stuff like satoshi-level rates
  const fractionDigits = ['USD', 'EUR', 'GBP', 'CNY', 'TWD', 'JPY'].includes(currency) ? 2 : 4
  return `${n.toLocaleString(undefined, {
    minimumFractionDigits: fractionDigits,
    maximumFractionDigits: fractionDigits,
  })} ${currency}`
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function EnergyBill() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  const [dayFrom, setDayFrom] = useState(daysAgoISO(30))
  const [dayTo, setDayTo] = useState(todayISO())

  const billQ = useEnergyBill(dayFrom, dayTo)
  const aggregate = useAggregateEnergyRange()

  // Asset lookup so we can show names instead of UUIDs.
  const assetsQ = useAssets({ page_size: '500' })
  const assetById = useMemo(() => {
    const m = new Map<string, { name: string; tag: string }>()
    for (const a of assetsQ.data?.data ?? []) {
      m.set(a.id, { name: a.name, tag: a.asset_tag ?? '' })
    }
    return m
  }, [assetsQ.data])

  const onAggregate = () => {
    if (!window.confirm(t('energy_bill.confirm_aggregate', { from: dayFrom, to: dayTo }))) return
    aggregate.mutate({ dayFrom, dayTo }, {
      onSuccess: (resp) => {
        const count = resp.data?.aggregated_count ?? 0
        toast.success(t('energy_bill.toast_aggregated', { count }))
      },
      onError: (e: unknown) => toast.error(e instanceof Error ? e.message : t('common.unknown_error')),
    })
  }

  const bill = billQ.data?.data
  const lines = bill?.lines ?? []

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      <header className="px-8 pt-6 pb-4">
        <nav className="flex items-center gap-1.5 text-xs text-on-surface-variant mb-3">
          <span className="hover:text-primary cursor-pointer" onClick={() => navigate('/monitoring/energy')}>
            {t('energy_bill.breadcrumb_energy')}
          </span>
          <Icon name="chevron_right" className="text-[14px] text-on-surface-variant" />
          <span className="text-primary">{t('energy_bill.title')}</span>
        </nav>

        <div className="flex items-start justify-between flex-wrap gap-4">
          <div>
            <h1 className="font-headline font-bold text-2xl text-on-surface">{t('energy_bill.title')}</h1>
            <p className="text-sm text-on-surface-variant mt-1">{t('energy_bill.subtitle')}</p>
          </div>
          <button
            onClick={() => navigate('/monitoring/energy/tariffs')}
            className="flex items-center gap-2 px-4 py-2 rounded-lg bg-surface-container-high text-on-surface text-sm font-semibold hover:bg-surface-container-highest transition-colors"
          >
            <Icon name="receipt" className="text-[18px]" />
            {t('energy_bill.btn_manage_tariffs')}
          </button>
        </div>
      </header>

      {/* Date range + actions */}
      <section className="px-8 pb-4">
        <div className="bg-surface-container rounded-lg p-5">
          <div className="flex flex-wrap items-end gap-4">
            <div>
              <label className="block text-xs text-on-surface-variant mb-1">{t('energy_bill.field_day_from')}</label>
              <input
                type="date"
                value={dayFrom}
                onChange={(e) => setDayFrom(e.target.value)}
                className="bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none"
              />
            </div>
            <div>
              <label className="block text-xs text-on-surface-variant mb-1">{t('energy_bill.field_day_to')}</label>
              <input
                type="date"
                value={dayTo}
                onChange={(e) => setDayTo(e.target.value)}
                className="bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none"
              />
            </div>
            <div className="flex flex-col gap-1">
              <span className="text-xs text-on-surface-variant">{t('energy_bill.quick_select')}</span>
              <div className="flex gap-1">
                {[
                  { key: 'last_7', days: 7 },
                  { key: 'last_30', days: 30 },
                  { key: 'last_90', days: 90 },
                ].map(({ key, days }) => (
                  <button
                    key={key}
                    onClick={() => {
                      setDayFrom(daysAgoISO(days))
                      setDayTo(todayISO())
                    }}
                    className="px-3 py-1.5 rounded-md text-xs bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest transition-colors"
                  >
                    {t(`energy_bill.${key}`)}
                  </button>
                ))}
              </div>
            </div>
            <div className="ml-auto">
              <button
                onClick={onAggregate}
                disabled={aggregate.isPending}
                className="flex items-center gap-2 px-4 py-2.5 rounded-lg bg-amber-500/20 text-amber-400 hover:bg-amber-500/30 text-sm font-semibold transition-colors disabled:opacity-40"
                title={t('energy_bill.aggregate_hint')}
              >
                <Icon name="cached" className="text-[18px]" />
                {aggregate.isPending ? t('common.saving') : t('energy_bill.btn_run_aggregate')}
              </button>
            </div>
          </div>
        </div>
      </section>

      {/* Summary */}
      <section className="px-8 pb-4">
        {billQ.isLoading && (
          <div className="bg-surface-container rounded-lg p-10 flex justify-center">
            <div className="animate-spin rounded-full h-6 w-6 border-2 border-sky-400 border-t-transparent" />
          </div>
        )}
        {billQ.error && (
          <div className="rounded-lg bg-red-900/20 p-4 text-red-300 text-sm">
            {t('energy_bill.load_failed')}{' '}
            <button onClick={() => billQ.refetch()} className="underline">{t('common.retry')}</button>
          </div>
        )}
        {bill && (
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="bg-surface-container rounded-lg p-5">
              <p className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant mb-1">
                {t('energy_bill.summary_total_kwh')}
              </p>
              <p className="font-headline text-2xl font-bold text-on-surface">{formatDecimal(bill.total_kwh, 2)}</p>
              <p className="text-xs text-on-surface-variant mt-1">
                {bill.day_from} → {bill.day_to}
              </p>
            </div>
            <div className="bg-surface-container rounded-lg p-5 md:col-span-2">
              <p className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant mb-1">
                {t('energy_bill.summary_total_cost')}
              </p>
              {bill.currency_mixed ? (
                <>
                  <p className="font-headline text-xl font-bold text-amber-400">
                    {t('energy_bill.mixed_currencies_label')}
                  </p>
                  <p className="text-xs text-on-surface-variant mt-1">
                    {t('energy_bill.mixed_currencies_hint')}
                  </p>
                </>
              ) : (
                <>
                  <p className="font-headline text-2xl font-bold text-emerald-400">
                    {formatCurrency(bill.total_cost, bill.currency)}
                  </p>
                  <p className="text-xs text-on-surface-variant mt-1">
                    {lines.length} {t('energy_bill.assets_billed')}
                  </p>
                </>
              )}
            </div>
          </div>
        )}
      </section>

      {/* Per-asset breakdown */}
      <section className="px-8 pb-8">
        <div className="bg-surface-container rounded-lg overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-surface-container-high text-on-surface-variant text-[0.6875rem] uppercase tracking-wider">
                <th className="px-4 py-3 text-left font-semibold">{t('energy_bill.col_asset')}</th>
                <th className="px-4 py-3 text-right font-semibold">{t('energy_bill.col_kwh')}</th>
                <th className="px-4 py-3 text-right font-semibold">{t('energy_bill.col_rate')}</th>
                <th className="px-4 py-3 text-right font-semibold">{t('energy_bill.col_cost')}</th>
              </tr>
            </thead>
            <tbody>
              {lines.length === 0 && bill && !billQ.isLoading && (
                <tr><td colSpan={4} className="py-10 text-center text-on-surface-variant text-sm">
                  {t('energy_bill.empty_state')}
                </td></tr>
              )}
              {lines.map((line) => {
                const meta = assetById.get(line.asset_id)
                return (
                  <tr key={line.asset_id} className="border-t border-surface-container-high">
                    <td className="px-4 py-3">
                      <button
                        onClick={() => navigate(`/assets/${line.asset_id}`)}
                        className="text-primary font-medium hover:underline text-left"
                      >
                        {meta?.name ?? line.asset_id.slice(0, 8) + '…'}
                      </button>
                      {meta?.tag && (
                        <p className="text-[0.6875rem] text-on-surface-variant font-mono mt-0.5">{meta.tag}</p>
                      )}
                    </td>
                    <td className="px-4 py-3 text-right font-mono">{formatDecimal(line.kwh, 2)}</td>
                    <td className="px-4 py-3 text-right font-mono text-on-surface-variant">
                      {line.rate_per_kwh} {line.currency}/kWh
                    </td>
                    <td className="px-4 py-3 text-right font-mono font-semibold text-emerald-400">
                      {formatCurrency(line.cost, line.currency)}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  )
}
