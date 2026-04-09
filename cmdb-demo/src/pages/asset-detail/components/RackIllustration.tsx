export default function RackIllustration() {
  return (
    <div className="bg-surface-container-low rounded-lg p-4 flex items-center justify-center min-h-[140px]">
      <svg width="72" height="112" viewBox="0 0 72 112" fill="none" role="img" aria-label="Rack illustration">
        <rect x="8" y="4" width="56" height="104" rx="3" stroke="#44474c" strokeWidth="1.5" fill="none" />
        {Array.from({ length: 10 }).map((_, i) => {
          const y = 10 + i * 10
          const isHighlighted = i === 6 || i === 7
          return <rect key={i} x="14" y={y} width="44" height="8" rx="1" fill={isHighlighted ? '#0087df' : '#202b32'} opacity={isHighlighted ? 0.8 : 1} />
        })}
        <text x="64" y="80" fill="#9ecaff" fontSize="6" fontFamily="Inter" textAnchor="end">U14</text>
      </svg>
    </div>
  )
}
