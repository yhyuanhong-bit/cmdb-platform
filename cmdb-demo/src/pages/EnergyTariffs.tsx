import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import Icon from '../components/Icon'
import {
  useEnergyTariffs,
  useCreateEnergyTariff,
  useUpdateEnergyTariff,
  useDeleteEnergyTariff,
} from '../hooks/useEnergyBilling'
import { useAllLocations } from '../hooks/useTopology'
import type { EnergyTariff, CreateTariffInput, UpdateTariffInput } from '../lib/api/energyBilling'

/* ------------------------------------------------------------------ */
/*  Tariff form dialog — used for both create and edit                 */
/* ------------------------------------------------------------------ */

interface TariffFormState {
  locationId: string // '' = tenant default
  currency: string
  ratePerKwh: string
  effectiveFrom: string
  effectiveTo: string
  notes: string
}

interface TariffDialogProps {
  open: boolean
  mode: 'create' | 'edit'
  initial: TariffFormState
  locations: Array<{ id: string; name: string }>
  onClose: () => void
  onSubmit: (state: TariffFormState) => void
  submitting: boolean
}

function TariffDialog({ open, mode, initial, locations, onClose, onSubmit, submitting }: TariffDialogProps) {
  const { t } = useTranslation()
  const [state, setState] = useState<TariffFormState>(initial)

  if (!open) return null

  const disabled =
    submitting ||
    state.ratePerKwh.trim() === '' ||
    state.effectiveFrom.trim() === '' ||
    Number(state.ratePerKwh) <= 0

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" role="dialog" aria-modal="true">
      <div className="bg-surface-container rounded-lg max-w-lg w-full p-6 shadow-xl max-h-[90vh] overflow-y-auto">
        <h2 className="font-headline font-bold text-lg text-on-surface mb-2">
          {mode === 'create' ? t('energy_tariffs.dialog_create_title') : t('energy_tariffs.dialog_edit_title')}
        </h2>
        <p className="text-sm text-on-surface-variant mb-4">
          {t('energy_tariffs.dialog_desc')}
        </p>

        <label className="block text-xs text-on-surface-variant mb-1">{t('energy_tariffs.field_location')}</label>
        <select
          value={state.locationId}
          onChange={(e) => setState((s) => ({ ...s, locationId: e.target.value }))}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none mb-3"
          disabled={mode === 'edit'} // locality is immutable per backend rule
        >
          <option value="">{t('energy_tariffs.option_tenant_default')}</option>
          {locations.map((l) => (
            <option key={l.id} value={l.id}>{l.name}</option>
          ))}
        </select>
        {mode === 'edit' && (
          <p className="text-[0.6875rem] text-on-surface-variant italic -mt-2 mb-3">
            {t('energy_tariffs.locality_immutable')}
          </p>
        )}

        <div className="grid grid-cols-2 gap-3 mb-3">
          <div>
            <label className="block text-xs text-on-surface-variant mb-1">{t('energy_tariffs.field_currency')}</label>
            <input
              value={state.currency}
              onChange={(e) => setState((s) => ({ ...s, currency: e.target.value.toUpperCase() }))}
              className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none uppercase"
              maxLength={3}
            />
          </div>
          <div>
            <label className="block text-xs text-on-surface-variant mb-1">{t('energy_tariffs.field_rate_per_kwh')}</label>
            <input
              type="text"
              inputMode="decimal"
              value={state.ratePerKwh}
              onChange={(e) => setState((s) => ({ ...s, ratePerKwh: e.target.value }))}
              className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none font-mono"
              placeholder="0.10"
            />
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3 mb-3">
          <div>
            <label className="block text-xs text-on-surface-variant mb-1">{t('energy_tariffs.field_effective_from')}</label>
            <input
              type="date"
              value={state.effectiveFrom}
              onChange={(e) => setState((s) => ({ ...s, effectiveFrom: e.target.value }))}
              className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none"
            />
          </div>
          <div>
            <label className="block text-xs text-on-surface-variant mb-1">
              {t('energy_tariffs.field_effective_to')}
              <span className="ml-1 normal-case text-[0.6rem] opacity-60">{t('energy_tariffs.optional_open_ended')}</span>
            </label>
            <input
              type="date"
              value={state.effectiveTo}
              onChange={(e) => setState((s) => ({ ...s, effectiveTo: e.target.value }))}
              className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none"
            />
          </div>
        </div>

        <label className="block text-xs text-on-surface-variant mb-1">{t('energy_tariffs.field_notes')}</label>
        <textarea
          value={state.notes}
          onChange={(e) => setState((s) => ({ ...s, notes: e.target.value }))}
          rows={2}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-3 outline-none resize-none"
          placeholder={t('energy_tariffs.placeholder_notes')}
        />

        <div className="flex justify-end gap-2 mt-4">
          <button
            onClick={onClose}
            disabled={submitting}
            className="px-4 py-2 rounded-lg text-sm text-on-surface-variant hover:bg-surface-container-high transition-colors"
          >
            {t('common.cancel')}
          </button>
          <button
            onClick={() => onSubmit(state)}
            disabled={disabled}
            className="px-4 py-2 rounded-lg text-sm font-semibold bg-primary text-on-primary hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed"
          >
            {submitting ? t('common.saving') : t('common.save')}
          </button>
        </div>
      </div>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

const today = () => new Date().toISOString().slice(0, 10)

export default function EnergyTariffs() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const tariffsQ = useEnergyTariffs()
  const create = useCreateEnergyTariff()
  const update = useUpdateEnergyTariff()
  const remove = useDeleteEnergyTariff()
  const locationsQ = useAllLocations()

  const tariffs = tariffsQ.data?.data ?? []
  const locations = (locationsQ.data?.data ?? []).map((l) => ({
    id: l.id,
    name: `${l.name} (${l.level})`,
  }))
  const locationName = (id?: string | null) => {
    if (!id) return t('energy_tariffs.tenant_default')
    return locations.find((l) => l.id === id)?.name ?? id.slice(0, 8) + '…'
  }

  const [createOpen, setCreateOpen] = useState(false)
  const [editing, setEditing] = useState<EnergyTariff | null>(null)

  const handleApiError = (e: unknown, fallbackKey: string) => {
    const msg = e instanceof Error ? e.message : ''
    if (msg.toLowerCase().includes('overlap')) {
      toast.error(t('energy_tariffs.toast_overlap'))
      return
    }
    toast.error(msg || t(fallbackKey))
  }

  const handleCreate = (s: TariffFormState) => {
    const body: CreateTariffInput = {
      location_id: s.locationId || null,
      currency: s.currency || 'USD',
      rate_per_kwh: s.ratePerKwh.trim(),
      effective_from: s.effectiveFrom,
      effective_to: s.effectiveTo || null,
      notes: s.notes || undefined,
    }
    create.mutate(body, {
      onSuccess: () => {
        toast.success(t('energy_tariffs.toast_created'))
        setCreateOpen(false)
      },
      onError: (e: unknown) => handleApiError(e, 'common.unknown_error'),
    })
  }

  const handleEdit = (s: TariffFormState) => {
    if (!editing) return
    const body: UpdateTariffInput = {
      currency: s.currency,
      rate_per_kwh: s.ratePerKwh.trim(),
      effective_from: s.effectiveFrom,
    }
    if (s.effectiveTo) {
      body.effective_to = s.effectiveTo
    } else {
      body.clear_effective_to = true
    }
    if (s.notes !== '') body.notes = s.notes
    update.mutate({ id: editing.id, body }, {
      onSuccess: () => {
        toast.success(t('energy_tariffs.toast_updated'))
        setEditing(null)
      },
      onError: (e: unknown) => handleApiError(e, 'common.unknown_error'),
    })
  }

  const handleDelete = (tariff: EnergyTariff) => {
    if (!window.confirm(t('energy_tariffs.confirm_delete'))) return
    remove.mutate(tariff.id, {
      onSuccess: () => toast.success(t('energy_tariffs.toast_deleted')),
      onError: (e: unknown) => handleApiError(e, 'common.unknown_error'),
    })
  }

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      <header className="px-8 pt-6 pb-4">
        <nav className="flex items-center gap-1.5 text-xs text-on-surface-variant mb-3">
          <span className="hover:text-primary cursor-pointer" onClick={() => navigate('/monitoring/energy')}>
            {t('energy_tariffs.breadcrumb_energy')}
          </span>
          <Icon name="chevron_right" className="text-[14px] text-on-surface-variant" />
          <span className="text-primary">{t('energy_tariffs.title')}</span>
        </nav>

        <div className="flex items-start justify-between flex-wrap gap-4">
          <div>
            <h1 className="font-headline font-bold text-2xl text-on-surface">{t('energy_tariffs.title')}</h1>
            <p className="text-sm text-on-surface-variant mt-1">{t('energy_tariffs.subtitle')}</p>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => navigate('/monitoring/energy/bill')}
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-surface-container-high text-on-surface text-sm font-semibold hover:bg-surface-container-highest transition-colors"
            >
              <Icon name="receipt_long" className="text-[18px]" />
              {t('energy_tariffs.btn_view_bill')}
            </button>
            <button
              onClick={() => setCreateOpen(true)}
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-on-primary text-sm font-semibold hover:opacity-90 transition-opacity"
            >
              <Icon name="add" className="text-[18px]" />
              {t('energy_tariffs.btn_new')}
            </button>
          </div>
        </div>
      </header>

      <section className="px-8 pb-8">
        <div className="bg-surface-container rounded-lg overflow-x-auto" role="table" aria-label="Tariff list">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-surface-container-high text-on-surface-variant text-[0.6875rem] uppercase tracking-wider">
                <th className="px-4 py-3 text-left font-semibold">{t('energy_tariffs.col_location')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('energy_tariffs.col_rate')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('energy_tariffs.col_effective')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('energy_tariffs.col_notes')}</th>
                <th className="px-4 py-3 text-right font-semibold">{t('common.actions')}</th>
              </tr>
            </thead>
            <tbody>
              {tariffsQ.isLoading && (
                <tr><td colSpan={5} className="py-10 text-center">
                  <div className="inline-block animate-spin rounded-full h-6 w-6 border-2 border-sky-400 border-t-transparent" />
                </td></tr>
              )}
              {tariffsQ.error && (
                <tr><td colSpan={5} className="py-4 text-center text-red-300 text-sm">
                  {t('energy_tariffs.load_failed')}{' '}
                  <button onClick={() => tariffsQ.refetch()} className="underline">{t('common.retry')}</button>
                </td></tr>
              )}
              {!tariffsQ.isLoading && !tariffsQ.error && tariffs.length === 0 && (
                <tr><td colSpan={5} className="py-10 text-center text-on-surface-variant text-sm">
                  {t('energy_tariffs.empty_state')}
                </td></tr>
              )}
              {tariffs.map((tariff) => {
                const expired = tariff.effective_to ? new Date(tariff.effective_to) < new Date() : false
                return (
                  <tr
                    key={tariff.id}
                    className={`border-t border-surface-container-high transition-colors ${expired ? 'opacity-60' : ''}`}
                  >
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        {!tariff.location_id ? (
                          <span className="px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider bg-blue-500/20 text-blue-400">
                            {t('energy_tariffs.tenant_default')}
                          </span>
                        ) : (
                          <span className="text-on-surface">{locationName(tariff.location_id)}</span>
                        )}
                      </div>
                    </td>
                    <td className="px-4 py-3 font-mono">
                      {tariff.rate_per_kwh}{' '}
                      <span className="text-on-surface-variant text-[0.6875rem] uppercase">{tariff.currency}/kWh</span>
                    </td>
                    <td className="px-4 py-3 text-xs text-on-surface-variant">
                      {tariff.effective_from} → {tariff.effective_to ?? t('energy_tariffs.open_ended')}
                    </td>
                    <td className="px-4 py-3 text-xs text-on-surface-variant max-w-xs truncate">
                      {tariff.notes ?? '—'}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <button
                        onClick={() => setEditing(tariff)}
                        className="p-1.5 rounded-md hover:bg-surface-container-high transition-colors"
                        aria-label={t('common.edit')}
                      >
                        <Icon name="edit" className="text-[18px] text-primary" />
                      </button>
                      <button
                        onClick={() => handleDelete(tariff)}
                        disabled={remove.isPending}
                        className="ml-1 p-1.5 rounded-md hover:bg-error-container/40 transition-colors disabled:opacity-40"
                        aria-label={t('common.delete')}
                      >
                        <Icon name="delete" className="text-[18px] text-error" />
                      </button>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      </section>

      <TariffDialog
        open={createOpen}
        mode="create"
        initial={{
          locationId: '',
          currency: 'USD',
          ratePerKwh: '',
          effectiveFrom: today(),
          effectiveTo: '',
          notes: '',
        }}
        locations={locations}
        onClose={() => setCreateOpen(false)}
        onSubmit={handleCreate}
        submitting={create.isPending}
      />

      {editing && (
        <TariffDialog
          open={true}
          mode="edit"
          initial={{
            locationId: editing.location_id ?? '',
            currency: editing.currency,
            ratePerKwh: editing.rate_per_kwh,
            effectiveFrom: editing.effective_from,
            effectiveTo: editing.effective_to ?? '',
            notes: editing.notes ?? '',
          }}
          locations={locations}
          onClose={() => setEditing(null)}
          onSubmit={handleEdit}
          submitting={update.isPending}
        />
      )}
    </div>
  )
}
