import type { Edge, XYPosition } from '@xyflow/react'

import type { StepNode } from './graph'

// elkjs is ~1.4 MB; load it the first time someone asks for a layout.
let elkPromise: Promise<InstanceType<typeof import('elkjs/lib/elk.bundled.js').default>> | undefined
const getElk = () =>
  (elkPromise ??= import('elkjs/lib/elk.bundled.js').then(({ default: ELK }) => new ELK()))

const FALLBACK_SIZE = { width: 240, height: 76 }

/** Left-to-right layered layout: each dependency level gets its own column. */
export async function layoutGraph(nodes: StepNode[], edges: Edge[]) {
  const elk = await getElk()
  const graph = await elk.layout({
    id: 'root',
    layoutOptions: {
      'elk.algorithm': 'layered',
      'elk.direction': 'RIGHT',
      'elk.layered.spacing.nodeNodeBetweenLayers': '72',
      'elk.spacing.nodeNode': '32',
      'elk.layered.nodePlacement.strategy': 'BRANDES_KOEPF',
    },
    children: nodes.map((n) => ({
      id: n.id,
      width: n.measured?.width ?? FALLBACK_SIZE.width,
      height: n.measured?.height ?? FALLBACK_SIZE.height,
    })),
    edges: edges.map((e) => ({ id: e.id, sources: [e.source], targets: [e.target] })),
  })

  return new Map<string, XYPosition>(
    (graph.children ?? []).map((c) => [c.id, { x: c.x ?? 0, y: c.y ?? 0 }]),
  )
}

/**
 * Moves nodes from their current positions to `to` over `duration` ms,
 * calling `apply` every frame.
 */
export function animatePositions(
  nodes: StepNode[],
  to: Map<string, XYPosition>,
  apply: (positions: Map<string, XYPosition>) => void,
  duration = 380,
) {
  const from = new Map(nodes.map((n) => [n.id, n.position]))
  const ease = (t: number) => 1 - Math.pow(1 - t, 4)
  const start = performance.now()

  return new Promise<void>((resolve) => {
    const frame = (now: number) => {
      const t = Math.min(1, (now - start) / duration)
      const k = ease(t)
      const step = new Map<string, XYPosition>()
      for (const [id, target] of to) {
        const origin = from.get(id) ?? target
        step.set(id, {
          x: origin.x + (target.x - origin.x) * k,
          y: origin.y + (target.y - origin.y) * k,
        })
      }
      apply(step)
      if (t < 1) requestAnimationFrame(frame)
      else resolve()
    }
    requestAnimationFrame(frame)
  })
}
