export default function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <h3 className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant mb-3">
      {children}
    </h3>
  )
}
