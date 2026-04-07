import { useMemo } from 'react'

export function useForceLayout(nodes: any[], edges: any[], width: number, height: number) {
  return useMemo(() => {
    if (!nodes.length) return []
    const positioned = nodes.map((n: any, i: number) => ({
      ...n,
      x: width / 2 + Math.cos((2 * Math.PI * i) / nodes.length) * Math.min(width, height) * 0.35,
      y: height / 2 + Math.sin((2 * Math.PI * i) / nodes.length) * Math.min(width, height) * 0.35,
    }))
    for (let iter = 0; iter < 100; iter++) {
      for (let i = 0; i < positioned.length; i++) {
        for (let j = i + 1; j < positioned.length; j++) {
          const dx = positioned[j].x - positioned[i].x
          const dy = positioned[j].y - positioned[i].y
          const dist = Math.max(Math.sqrt(dx * dx + dy * dy), 1)
          const force = 5000 / (dist * dist)
          const fx = (dx / dist) * force
          const fy = (dy / dist) * force
          positioned[i].x -= fx; positioned[i].y -= fy
          positioned[j].x += fx; positioned[j].y += fy
        }
      }
      const idxMap = new Map(positioned.map((n: any, i: number) => [n.id, i]))
      for (const edge of edges) {
        const si = idxMap.get(edge.from)
        const ti = idxMap.get(edge.to)
        if (si == null || ti == null) continue
        const s = positioned[si], t = positioned[ti]
        const dx = t.x - s.x, dy = t.y - s.y
        const dist = Math.sqrt(dx * dx + dy * dy)
        const force = (dist - 200) * 0.01
        const fx = (dx / Math.max(dist, 1)) * force
        const fy = (dy / Math.max(dist, 1)) * force
        s.x += fx; s.y += fy; t.x -= fx; t.y -= fy
      }
      for (const n of positioned) {
        n.x += (width / 2 - n.x) * 0.01
        n.y += (height / 2 - n.y) * 0.01
      }
    }
    for (const n of positioned) {
      n.x = Math.max(60, Math.min(width - 60, n.x))
      n.y = Math.max(60, Math.min(height - 60, n.y))
    }
    return positioned
  }, [nodes, edges, width, height])
}
