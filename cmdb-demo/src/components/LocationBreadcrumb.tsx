import { ReactNode } from 'react'
import { useLocationContext } from '../contexts/LocationContext'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'

interface LocationBreadcrumbProps {
  suffix?: ReactNode
}

export default function LocationBreadcrumb({ suffix }: LocationBreadcrumbProps) {
  const { breadcrumbs } = useLocationContext()
  const navigate = useNavigate()
  const { i18n } = useTranslation()

  const isEn = i18n.language === 'en'

  return (
    <nav
      aria-label="Location breadcrumb"
      className="flex items-center gap-1.5 text-xs uppercase tracking-widest text-on-surface-variant"
    >
      {/* Global root */}
      {breadcrumbs.length === 0 ? (
        <span className="text-on-surface font-semibold">Global</span>
      ) : (
        <>
          <span
            className="cursor-pointer hover:text-primary transition-colors"
            onClick={() => navigate('/locations')}
            role="link"
            tabIndex={0}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                navigate('/locations')
              }
            }}
          >
            Global
          </span>
          <Separator />
        </>
      )}

      {/* Path segments */}
      {breadcrumbs.map((crumb, idx) => {
        const isLast = idx === breadcrumbs.length - 1 && !suffix
        const displayLabel = isEn ? crumb.labelEn : crumb.label

        return (
          <span key={crumb.to} className="flex items-center gap-1.5">
            {isLast ? (
              <span className="text-on-surface font-semibold">
                {displayLabel}
              </span>
            ) : (
              <>
                <span
                  className="cursor-pointer hover:text-primary transition-colors"
                  onClick={() => navigate(crumb.to)}
                  role="link"
                  tabIndex={0}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault()
                      navigate(crumb.to)
                    }
                  }}
                >
                  {displayLabel}
                </span>
                <Separator />
              </>
            )}
          </span>
        )
      })}

      {/* Optional suffix */}
      {suffix && (
        <>
          {breadcrumbs.length > 0 && <Separator />}
          <span className="text-on-surface font-semibold">{suffix}</span>
        </>
      )}
    </nav>
  )
}

function Separator() {
  return (
    <span className="text-[10px] opacity-40" aria-hidden="true">
      ›
    </span>
  )
}
