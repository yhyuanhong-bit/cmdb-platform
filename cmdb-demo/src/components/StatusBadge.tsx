const colorMap: Record<string, string> = {
  critical: 'bg-error-container text-on-error-container',
  warning: 'bg-[#92400e] text-[#fbbf24]',
  info: 'bg-[#1e3a5f] text-on-primary-container',
  high: 'bg-error-container text-on-error-container',
  medium: 'bg-[#92400e] text-[#fbbf24]',
  low: 'bg-[#1e3a5f] text-primary',
  operational: 'bg-[#064e3b] text-[#34d399]',
  degraded: 'bg-[#92400e] text-[#fbbf24]',
  offline: 'bg-surface-container-highest text-on-surface-variant',
  open: 'bg-on-primary-container/20 text-on-primary-container',
  acknowledged: 'bg-[#92400e]/20 text-[#fbbf24]',
  resolved: 'bg-[#064e3b]/20 text-[#34d399]',
  in_progress: 'bg-on-primary-container/20 text-on-primary-container',
  completed: 'bg-[#064e3b] text-[#34d399]',
  pending: 'bg-[#92400e] text-[#fbbf24]',
  success: 'bg-[#064e3b] text-[#34d399]',
  maintenance: 'bg-[#92400e] text-[#fbbf24]',
}

export default function StatusBadge({ status }: { status: string }) {
  const key = status.toLowerCase().replace(/[\s-]/g, '_')
  const colors = colorMap[key] || 'bg-surface-container-highest text-on-surface-variant'
  return (
    <span className={`px-2.5 py-1 rounded text-[0.6875rem] font-semibold uppercase tracking-wider ${colors}`}>
      {status}
    </span>
  )
}
