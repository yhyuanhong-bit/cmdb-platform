import type { ReactNode } from 'react'
import Icon from './Icon'

interface EmptyStateProps {
  icon?: string
  title: string
  description?: string
  action?: ReactNode
  /** Visual tone — affects accent color and icon ring */
  tone?: 'neutral' | 'info' | 'warning'
  /** Compact variant uses less vertical space (for inline card placements) */
  compact?: boolean
  className?: string
}

const TONE_STYLES: Record<NonNullable<EmptyStateProps['tone']>, { ring: string; iconColor: string; bg: string }> = {
  neutral: {
    ring: 'ring-on-surface-variant/10',
    iconColor: 'text-on-surface-variant',
    bg: 'bg-surface-container-low',
  },
  info: {
    ring: 'ring-primary/20',
    iconColor: 'text-primary',
    bg: 'bg-primary-container/20',
  },
  warning: {
    ring: 'ring-[#ffa94d]/25',
    iconColor: 'text-[#ffa94d]',
    bg: 'bg-[#ffa94d]/10',
  },
}

/**
 * EmptyState — surface-level placeholder for "no data yet" scenarios.
 *
 * Design notes:
 * - Real hierarchy: iconographic badge → title → muted description → optional CTA
 * - Not a generic "uniform card": uses ring accent + subtle tinted badge to signal
 *   intent (neutral/info/warning) and avoid template-looking flat blocks.
 * - Respects surrounding surface container; caller controls outer spacing.
 */
export default function EmptyState({
  icon = 'inbox',
  title,
  description,
  action,
  tone = 'neutral',
  compact = false,
  className = '',
}: EmptyStateProps) {
  const s = TONE_STYLES[tone]
  return (
    <div
      className={`flex flex-col items-center justify-center text-center ${
        compact ? 'py-6 gap-2' : 'py-10 gap-3'
      } ${className}`}
    >
      <div
        className={`flex h-12 w-12 items-center justify-center rounded-xl ${s.bg} ring-1 ${s.ring}`}
        aria-hidden="true"
      >
        <Icon name={icon} className={`text-2xl ${s.iconColor}`} />
      </div>
      <h3 className="font-headline text-sm font-semibold tracking-wide text-on-surface">
        {title}
      </h3>
      {description && (
        <p className="max-w-sm text-xs leading-relaxed text-on-surface-variant">
          {description}
        </p>
      )}
      {action && <div className="mt-1">{action}</div>}
    </div>
  )
}
