import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  ASSET_LIFESPAN_DEFAULTS,
  ASSET_LIFESPAN_MAX,
  ASSET_LIFESPAN_MIN,
  type AssetLifespanConfig,
} from '../lib/api/settings'
import {
  useAssetLifespanSettings,
  useUpdateAssetLifespan,
} from '../hooks/useSettings'

type LifespanField = keyof Required<AssetLifespanConfig>

const FIELDS: ReadonlyArray<{ field: LifespanField; labelKey: string }> = [
  { field: 'server', labelKey: 'system_settings.lifespan_server' },
  { field: 'network', labelKey: 'system_settings.lifespan_network' },
  { field: 'storage', labelKey: 'system_settings.lifespan_storage' },
  { field: 'power', labelKey: 'system_settings.lifespan_power' },
]

type FormState = Record<LifespanField, string>
type FieldErrors = Partial<Record<LifespanField, string>>

function toFormState(config: AssetLifespanConfig | undefined): FormState {
  const merged = { ...ASSET_LIFESPAN_DEFAULTS, ...(config ?? {}) }
  return {
    server: String(merged.server),
    network: String(merged.network),
    storage: String(merged.storage),
    power: String(merged.power),
  }
}

function defaultsAsForm(): FormState {
  return toFormState(undefined)
}

function validateField(value: string): string | null {
  if (value.trim() === '') return 'required'
  const n = Number(value)
  if (!Number.isInteger(n)) return 'integer'
  if (n < ASSET_LIFESPAN_MIN || n > ASSET_LIFESPAN_MAX) return 'range'
  return null
}

export default function AssetLifespanSection() {
  const { t } = useTranslation()
  const { data, isLoading, isError } = useAssetLifespanSettings()
  const update = useUpdateAssetLifespan()

  const initialForm = useMemo(() => toFormState(data?.data), [data])
  const [form, setForm] = useState<FormState>(initialForm)
  const [errors, setErrors] = useState<FieldErrors>({})

  // Re-sync local form when fresh server data arrives (load or post-save).
  useEffect(() => {
    setForm(initialForm)
    setErrors({})
  }, [initialForm])

  const handleChange = (field: LifespanField, value: string) => {
    setForm((prev) => ({ ...prev, [field]: value }))
    setErrors((prev) => {
      const errKey = validateField(value)
      const next: FieldErrors = { ...prev }
      if (errKey) next[field] = errKey
      else delete next[field]
      return next
    })
  }

  const validateAll = (): FieldErrors => {
    const next: FieldErrors = {}
    for (const { field } of FIELDS) {
      const errKey = validateField(form[field])
      if (errKey) next[field] = errKey
    }
    return next
  }

  const handleReset = () => {
    setForm(defaultsAsForm())
    setErrors({})
  }

  const handleSave = () => {
    const v = validateAll()
    if (Object.keys(v).length > 0) {
      setErrors(v)
      toast.error(t('system_settings.lifespan_error'))
      return
    }
    const payload: Required<AssetLifespanConfig> = {
      server: Number(form.server),
      network: Number(form.network),
      storage: Number(form.storage),
      power: Number(form.power),
    }
    update.mutate(payload, {
      onSuccess: () => toast.success(t('system_settings.lifespan_saved')),
      onError: () => toast.error(t('system_settings.lifespan_error')),
    })
  }

  const errorMessage = (errKey: string | undefined): string | null => {
    if (!errKey) return null
    if (errKey === 'required' || errKey === 'integer' || errKey === 'range') {
      return t('system_settings.lifespan_validation_range', {
        min: ASSET_LIFESPAN_MIN,
        max: ASSET_LIFESPAN_MAX,
      })
    }
    return null
  }

  const hasErrors = Object.keys(errors).length > 0
  const saving = update.isPending

  return (
    <section
      aria-labelledby="asset-lifespan-heading"
      className="bg-surface-container rounded-lg p-6 mt-6"
    >
      <div className="flex items-start justify-between gap-4 mb-1">
        <div>
          <h2
            id="asset-lifespan-heading"
            className="font-headline font-bold text-lg text-on-surface"
          >
            {t('system_settings.lifespan_title')}
          </h2>
          <p className="text-on-surface-variant text-xs mt-1 max-w-2xl">
            {t('system_settings.lifespan_subtitle')}
          </p>
        </div>
      </div>

      {isLoading ? (
        <p className="text-sm text-on-surface-variant py-4">
          {t('system_settings.lifespan_loading')}
        </p>
      ) : isError ? (
        <p className="text-sm text-error py-4">
          {t('system_settings.lifespan_load_error')}
        </p>
      ) : (
        <>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mt-4">
            {FIELDS.map(({ field, labelKey }) => {
              const errKey = errors[field]
              const inputId = `lifespan-${field}`
              const errId = `${inputId}-error`
              return (
                <div key={field} className="flex flex-col gap-1.5">
                  <label
                    htmlFor={inputId}
                    className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label"
                  >
                    {t(labelKey)}
                  </label>
                  <input
                    id={inputId}
                    type="number"
                    inputMode="numeric"
                    min={ASSET_LIFESPAN_MIN}
                    max={ASSET_LIFESPAN_MAX}
                    step={1}
                    value={form[field]}
                    onChange={(e) => handleChange(field, e.target.value)}
                    aria-invalid={errKey ? true : undefined}
                    aria-describedby={errKey ? errId : undefined}
                    className={`w-full px-3 py-2 bg-surface-container-low rounded border text-on-surface ${
                      errKey
                        ? 'border-red-500/60 focus:outline-red-500'
                        : 'border-surface-container-highest focus:outline-on-primary-container'
                    }`}
                  />
                  <span className="text-[0.6875rem] text-on-surface-variant">
                    {t('system_settings.lifespan_unit_years')}
                  </span>
                  {errKey && (
                    <span id={errId} className="text-[0.6875rem] text-red-400">
                      {errorMessage(errKey)}
                    </span>
                  )}
                </div>
              )
            })}
          </div>

          <div className="flex items-center justify-end gap-2 mt-5">
            <button
              type="button"
              onClick={handleReset}
              disabled={saving}
              className="px-4 py-2 rounded-lg bg-surface-container-high text-on-surface-variant text-sm font-semibold hover:bg-surface-container-highest transition-colors disabled:opacity-50"
            >
              {t('system_settings.lifespan_reset')}
            </button>
            <button
              type="button"
              onClick={handleSave}
              disabled={saving || hasErrors}
              className="px-4 py-2 rounded-lg bg-on-primary-container text-white text-sm font-semibold hover:bg-on-primary-container/90 transition-colors disabled:opacity-50"
            >
              {saving
                ? t('system_settings.lifespan_saving')
                : t('system_settings.lifespan_save')}
            </button>
          </div>
        </>
      )}
    </section>
  )
}
