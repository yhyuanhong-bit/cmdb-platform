import { useCallback, useMemo } from 'react'
import { useSearchParams } from 'react-router-dom'

/**
 * useUrlState — persist a typed piece of UI state (filters, pagination, sort,
 * active tab, search query) in the URL via `useSearchParams`.
 *
 * Usage:
 *   const [filters, setFilters] = useUrlState('assets', {
 *     q: '',
 *     page: 1,
 *     status: 'all' as 'all' | 'online' | 'offline',
 *     tags: [] as string[],
 *   })
 *
 * Features:
 *   - Each field keeps its type (string | number | boolean | string[]).
 *     Enum fields are modeled as string literal unions with a typed default.
 *   - Values equal to their default are OMITTED from the URL, keeping URLs
 *     clean when the user hasn't customized anything.
 *   - Keys are namespaced as `<key>.<field>` so multiple hooks on the same
 *     page (e.g. `useUrlState('assets', ...)` + `useUrlState('racks', ...)`)
 *     don't collide, and unrelated query params are preserved on write.
 *   - Defaults to history-`replace` so typing in a filter doesn't spam the
 *     back button. Pass `{ mode: 'push' }` for explicit push navigation.
 *   - Malformed URL values fall back to defaults silently — never throws on
 *     user-pasted URLs.
 */
export type UrlStateValue = string | number | boolean | string[]
export type UrlStateDefaults = Record<string, UrlStateValue>

export interface UseUrlStateOptions {
  /**
   * History navigation mode for writes. Defaults to 'replace' so rapid input
   * changes (typing in a search field) don't generate a back-button trail.
   */
  mode?: 'push' | 'replace'
}

type Setter<T> = (next: Partial<T>) => void

function isStringArrayDefault(v: UrlStateValue): v is string[] {
  return Array.isArray(v)
}

/**
 * Deserialize a single raw URL value using the shape of its default.
 * Returns the default when the raw value is missing or malformed.
 */
function decode<V extends UrlStateValue>(raw: string | null, defaultValue: V): V {
  if (raw === null) return defaultValue

  if (typeof defaultValue === 'string') {
    return raw as V
  }

  if (typeof defaultValue === 'number') {
    const n = Number(raw)
    if (!Number.isFinite(n)) return defaultValue
    return n as V
  }

  if (typeof defaultValue === 'boolean') {
    if (raw === 'true') return true as V
    if (raw === 'false') return false as V
    return defaultValue
  }

  if (isStringArrayDefault(defaultValue)) {
    if (raw === '') return defaultValue
    return raw.split(',') as V
  }

  return defaultValue
}

/**
 * Serialize a value to a URL string, or return null to indicate the value
 * is equal to its default and should be omitted.
 */
function encode<V extends UrlStateValue>(value: V, defaultValue: V): string | null {
  if (isStringArrayDefault(defaultValue)) {
    const arr = value as string[]
    if (arr.length === 0) return null
    if (
      Array.isArray(defaultValue) &&
      arr.length === defaultValue.length &&
      arr.every((v, i) => v === defaultValue[i])
    ) {
      return null
    }
    return arr.join(',')
  }

  if (value === defaultValue) return null

  if (typeof value === 'boolean') return value ? 'true' : 'false'
  if (typeof value === 'number') return String(value)
  return String(value)
}

export function useUrlState<T extends UrlStateDefaults>(
  key: string,
  defaults: T,
  options: UseUrlStateOptions = {},
): [T, Setter<T>] {
  const { mode = 'replace' } = options
  const [searchParams, setSearchParams] = useSearchParams()

  const state = useMemo(() => {
    const out = {} as T
    for (const field of Object.keys(defaults) as Array<keyof T>) {
      const defaultValue = defaults[field]
      const paramKey = `${key}.${String(field)}`
      const raw = searchParams.get(paramKey)
      out[field] = decode(raw, defaultValue) as T[keyof T]
    }
    return out
    // defaults object is expected to be stable (declared inline or memoized
    // by the caller), but we intentionally depend on searchParams so we
    // re-derive when the URL changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams, key])

  const setState = useCallback<Setter<T>>(
    (next) => {
      setSearchParams(
        (prev) => {
          const params = new URLSearchParams(prev)
          for (const field of Object.keys(next) as Array<keyof T>) {
            const value = next[field]
            if (value === undefined) continue
            const defaultValue = defaults[field]
            const paramKey = `${key}.${String(field)}`
            const encoded = encode(value as UrlStateValue, defaultValue)
            if (encoded === null) {
              params.delete(paramKey)
            } else {
              params.set(paramKey, encoded)
            }
          }
          return params
        },
        { replace: mode === 'replace' },
      )
    },
    // defaults is expected to be stable — pages typically declare it at
    // module scope or via useMemo. We depend on key/setSearchParams/mode.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [key, setSearchParams, mode],
  )

  return [state, setState]
}
