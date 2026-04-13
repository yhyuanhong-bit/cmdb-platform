import Icon from './Icon'

interface StatCardProps {
  icon: string
  label: string
  value: string | number
  sub?: string
  subColor?: string
}

export default function StatCard({ icon, label, value, sub, subColor = 'text-on-primary-container' }: StatCardProps) {
  return (
    <div className="bg-surface-container-low rounded-lg p-5 flex flex-col gap-2">
      <div className="flex items-center justify-between">
        <span className="font-label text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant">{label}</span>
        <Icon name={icon} className="text-on-surface-variant text-[18px]" />
      </div>
      <div className="font-headline font-bold text-2xl text-on-surface">{value}</div>
      {sub && <span className={`text-xs ${subColor}`}>{sub}</span>}
    </div>
  )
}
