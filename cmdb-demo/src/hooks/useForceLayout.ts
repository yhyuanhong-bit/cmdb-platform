import { useMemo } from 'react'

interface ForceNode { id: string; [key: string]: unknown }
interface ForceEdge { from: string; to: string; [key: string]: unknown }

export function useForceLayout(nodes: ForceNode[], edges: ForceEdge[], width: number, height: number) {
  return useMemo(() => {
    if (!nodes.length) return []

    const positioned = nodes.map((n, i) => ({
      ...n,
      x: width / 2 + Math.cos((2 * Math.PI * i) / nodes.length) * Math.min(width, height) * 0.35,
      y: height / 2 + Math.sin((2 * Math.PI * i) / nodes.length) * Math.min(width, height) * 0.35,
    }))

    const iterations = Math.min(50, Math.max(20, Math.round(200 / nodes.length)))
    const cellSize = 120

    for (let iter = 0; iter < iterations; iter++) {
      let maxDisplacement = 0

      // Grid-based spatial hashing for repulsion
      const grid = new Map<string, number[]>()
      for (let i = 0; i < positioned.length; i++) {
        const cx = Math.floor(positioned[i].x / cellSize)
        const cy = Math.floor(positioned[i].y / cellSize)
        const key = `${cx},${cy}`
        if (!grid.has(key)) grid.set(key, [])
        grid.get(key)!.push(i)
      }

      // Repulsion: only check neighboring cells
      for (let i = 0; i < positioned.length; i++) {
        const cx = Math.floor(positioned[i].x / cellSize)
        const cy = Math.floor(positioned[i].y / cellSize)

        for (let dcx = -1; dcx <= 1; dcx++) {
          for (let dcy = -1; dcy <= 1; dcy++) {
            const neighbors = grid.get(`${cx + dcx},${cy + dcy}`)
            if (!neighbors) continue

            for (const j of neighbors) {
              if (j <= i) continue
              const dx = positioned[j].x - positioned[i].x
              const dy = positioned[j].y - positioned[i].y
              const dist = Math.max(Math.sqrt(dx * dx + dy * dy), 1)
              const force = 5000 / (dist * dist)
              const fx = (dx / dist) * force
              const fy = (dy / dist) * force
              positioned[i].x -= fx
              positioned[i].y -= fy
              positioned[j].x += fx
              positioned[j].y += fy
              maxDisplacement = Math.max(maxDisplacement, Math.abs(fx), Math.abs(fy))
            }
          }
        }
      }

      // Attraction along edges
      const idxMap = new Map(positioned.map((n, i) => [n.id, i]))
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
        s.x += fx; s.y += fy
        t.x -= fx; t.y -= fy
      }

      // Centering force
      for (const n of positioned) {
        n.x += (width / 2 - n.x) * 0.01
        n.y += (height / 2 - n.y) * 0.01
      }

      // Early exit if converged
      if (maxDisplacement < 0.5) break
    }

    // Boundary clamping
    for (const n of positioned) {
      n.x = Math.max(60, Math.min(width - 60, n.x))
      n.y = Math.max(60, Math.min(height - 60, n.y))
    }

    return positioned
  }, [nodes, edges, width, height])
}
