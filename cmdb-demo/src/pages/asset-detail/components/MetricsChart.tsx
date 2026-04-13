export function toSvgPath(
  points: { t: number; value: number }[],
  width: number,
  height: number,
  padding = 16,
): string {
  const maxT = Math.max(...points.map((p) => p.t))
  const maxV = 100
  const xScale = (width - padding * 2) / (maxT || 1)
  const yScale = (height - padding * 2) / maxV
  return points
    .map((p, i) => {
      const x = padding + p.t * xScale
      const y = height - padding - p.value * yScale
      return `${i === 0 ? 'M' : 'L'}${x},${y}`
    })
    .join(' ')
}
