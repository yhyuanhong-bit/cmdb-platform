import { useMemo } from 'react'

interface ForceNode { id: string; [key: string]: unknown }
interface ForceEdge { from: string; to: string; [key: string]: unknown }

export function useForceLayout(
  nodes: ForceNode[],
  edges: ForceEdge[],
  width: number,
  height: number,
  nodeWidth = 0,
  nodeHeight = 0,
) {
  return useMemo(() => {
    if (!nodes.length) return []

    // Account for node dimensions in layout calculations
    const nw = nodeWidth || 0
    const nh = nodeHeight || 0
    const padX = Math.max(60, nw / 2 + 10)
    const padY = Math.max(60, nh / 2 + 10)

    // Minimum separation based on node size
    const minSepX = nw > 0 ? nw + 20 : 100
    const minSepY = nh > 0 ? nh + 16 : 60
    const minSep = Math.sqrt(minSepX * minSepX + minSepY * minSepY)

    // Ideal edge length scales with node size
    const idealEdgeLen = Math.max(minSep * 1.3, 250)

    // Repulsion strength scales with node area
    const repulsionK = nw > 0 ? minSep * minSep * 2 : 5000

    const positioned = nodes.map((n, i) => ({
      ...n,
      x: width / 2 + Math.cos((2 * Math.PI * i) / nodes.length) * Math.min(width - padX * 2, height - padY * 2) * 0.4,
      y: height / 2 + Math.sin((2 * Math.PI * i) / nodes.length) * Math.min(width - padX * 2, height - padY * 2) * 0.4,
    }))

    const iterations = Math.min(80, Math.max(30, Math.round(300 / nodes.length)))
    const cellSize = Math.max(minSep, 120)

    for (let iter = 0; iter < iterations; iter++) {
      let maxDisplacement = 0
      const damping = 1 - iter / iterations * 0.5 // cool down over time

      // Grid-based spatial hashing for repulsion
      const grid = new Map<string, number[]>()
      for (let i = 0; i < positioned.length; i++) {
        const cx = Math.floor(positioned[i].x / cellSize)
        const cy = Math.floor(positioned[i].y / cellSize)
        const key = `${cx},${cy}`
        if (!grid.has(key)) grid.set(key, [])
        grid.get(key)!.push(i)
      }

      // Repulsion: check neighboring cells (range 2 for larger nodes)
      const range = nw > 100 ? 2 : 1
      for (let i = 0; i < positioned.length; i++) {
        const cx = Math.floor(positioned[i].x / cellSize)
        const cy = Math.floor(positioned[i].y / cellSize)

        for (let dcx = -range; dcx <= range; dcx++) {
          for (let dcy = -range; dcy <= range; dcy++) {
            const neighbors = grid.get(`${cx + dcx},${cy + dcy}`)
            if (!neighbors) continue

            for (const j of neighbors) {
              if (j <= i) continue
              const dx = positioned[j].x - positioned[i].x
              const dy = positioned[j].y - positioned[i].y
              const dist = Math.max(Math.sqrt(dx * dx + dy * dy), 1)

              // Stronger repulsion when nodes overlap
              const overlap = dist < minSep
              const force = overlap
                ? repulsionK / (dist * dist) * 3
                : repulsionK / (dist * dist)

              const fx = (dx / dist) * force * damping
              const fy = (dy / dist) * force * damping
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
      const idxMap = new Map(positioned.map((n, idx) => [n.id, idx]))
      for (const edge of edges) {
        const si = idxMap.get(edge.from)
        const ti = idxMap.get(edge.to)
        if (si == null || ti == null) continue
        const s = positioned[si], t = positioned[ti]
        const dx = t.x - s.x, dy = t.y - s.y
        const dist = Math.sqrt(dx * dx + dy * dy)
        const force = (dist - idealEdgeLen) * 0.008 * damping
        const fx = (dx / Math.max(dist, 1)) * force
        const fy = (dy / Math.max(dist, 1)) * force
        s.x += fx; s.y += fy
        t.x -= fx; t.y -= fy
      }

      // Centering force
      for (const n of positioned) {
        n.x += (width / 2 - n.x) * 0.005
        n.y += (height / 2 - n.y) * 0.005
      }

      // Early exit if converged
      if (maxDisplacement < 0.3) break
    }

    // Boundary clamping (account for node dimensions)
    for (const n of positioned) {
      n.x = Math.max(padX, Math.min(width - padX - nw, n.x))
      n.y = Math.max(padY, Math.min(height - padY - nh, n.y))
    }

    return positioned
  }, [nodes, edges, width, height, nodeWidth, nodeHeight])
}
