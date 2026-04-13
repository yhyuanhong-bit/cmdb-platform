export default function DataRow({
  label,
  value,
  mono,
  valueColor,
}: {
  label: string
  value: React.ReactNode
  mono?: boolean
  valueColor?: string
}) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">{label}</span>
      <span className={`text-sm ${valueColor ?? 'text-on-surface'} ${mono ? 'font-mono' : 'font-body'}`}>{value}</span>
    </div>
  )
}
