import {
  createContext,
  useContext,
  useState,
  useCallback,
  useMemo,
  ReactNode,
  useEffect,
} from 'react'

export interface LocationNode {
  id: string
  slug: string
  name: string
  nameEn: string
}

export interface LocationPath {
  country?: LocationNode
  region?: LocationNode
  city?: LocationNode
  campus?: LocationNode
  idc?: LocationNode
}

export type LocationLevel = 'global' | 'country' | 'region' | 'city' | 'campus' | 'idc'

interface LocationContextValue {
  path: LocationPath
  currentLevel: LocationLevel
  setPath: (path: LocationPath) => void
  navigateToLevel: (level: LocationLevel, node?: LocationNode) => void
  breadcrumbs: Array<{ label: string; labelEn: string; to: string }>
}

const STORAGE_KEY = 'irongrid_location'

const LocationContext = createContext<LocationContextValue | undefined>(undefined)

function deriveLevel(path: LocationPath): LocationLevel {
  if (path.idc) return 'idc'
  if (path.campus) return 'campus'
  if (path.city) return 'city'
  if (path.region) return 'region'
  if (path.country) return 'country'
  return 'global'
}

function loadPersistedPath(): LocationPath {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      return JSON.parse(raw) as LocationPath
    }
  } catch {
    // Ignore malformed data
  }
  return {}
}

function persistPath(path: LocationPath): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(path))
  } catch {
    // Ignore storage errors
  }
}

export function LocationProvider({ children }: { children: ReactNode }) {
  const [path, setPathState] = useState<LocationPath>(loadPersistedPath)

  useEffect(() => {
    persistPath(path)
  }, [path])

  const setPath = useCallback((next: LocationPath) => {
    setPathState(next)
  }, [])

  const navigateToLevel = useCallback(
    (level: LocationLevel, node?: LocationNode) => {
      setPathState((prev) => {
        switch (level) {
          case 'global':
            return {}
          case 'country':
            return { country: node ?? prev.country }
          case 'region':
            return { country: prev.country, region: node ?? prev.region }
          case 'city':
            return {
              country: prev.country,
              region: prev.region,
              city: node ?? prev.city,
            }
          case 'campus':
            return {
              country: prev.country,
              region: prev.region,
              city: prev.city,
              campus: node ?? prev.campus,
            }
          case 'idc':
            return {
              country: prev.country,
              region: prev.region,
              city: prev.city,
              campus: prev.campus,
              idc: node ?? prev.idc,
            }
          default:
            return prev
        }
      })
    },
    [],
  )

  const currentLevel = useMemo(() => deriveLevel(path), [path])

  const breadcrumbs = useMemo(() => {
    const crumbs: Array<{ label: string; labelEn: string; to: string }> = []

    if (path.country) {
      crumbs.push({
        label: path.country.name,
        labelEn: path.country.nameEn,
        to: `/locations/${path.country.slug}`,
      })
    }

    if (path.region && path.country) {
      crumbs.push({
        label: path.region.name,
        labelEn: path.region.nameEn,
        to: `/locations/${path.country.slug}/${path.region.slug}`,
      })
    }

    if (path.city && path.country && path.region) {
      crumbs.push({
        label: path.city.name,
        labelEn: path.city.nameEn,
        to: `/locations/${path.country.slug}/${path.region.slug}/${path.city.slug}`,
      })
    }

    if (path.campus && path.country && path.region && path.city) {
      crumbs.push({
        label: path.campus.name,
        labelEn: path.campus.nameEn,
        to: `/locations/${path.country.slug}/${path.region.slug}/${path.city.slug}/${path.campus.slug}`,
      })
    }

    if (path.idc && path.country && path.region && path.city && path.campus) {
      crumbs.push({
        label: path.idc.name,
        labelEn: path.idc.nameEn,
        to: '/dashboard',
      })
    }

    return crumbs
  }, [path])

  const value = useMemo<LocationContextValue>(
    () => ({
      path,
      currentLevel,
      setPath,
      navigateToLevel,
      breadcrumbs,
    }),
    [path, currentLevel, setPath, navigateToLevel, breadcrumbs],
  )

  return (
    <LocationContext.Provider value={value}>{children}</LocationContext.Provider>
  )
}

export function useLocationContext(): LocationContextValue {
  const ctx = useContext(LocationContext)
  if (!ctx) {
    throw new Error('useLocationContext must be used within a LocationProvider')
  }
  return ctx
}
