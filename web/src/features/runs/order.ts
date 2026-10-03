import type { Definition } from '@/features/workflows/api'

/**
 * Steps in execution order: by dependency depth, then by their order in the
 * definition. Steps with the same depth can run in parallel.
 */
export function executionOrder(def: Definition): { id: string; depth: number; after: string[] }[] {
  const after = new Map<string, string[]>(def.steps.map((s) => [s.id, []]))
  for (const e of def.edges) after.get(e.to)?.push(e.from)

  const depth = new Map<string, number>()
  const depthOf = (id: string): number => {
    const known = depth.get(id)
    if (known !== undefined) return known
    const deps = after.get(id) ?? []
    const d = deps.length === 0 ? 0 : 1 + Math.max(...deps.map(depthOf))
    depth.set(id, d)
    return d
  }

  return def.steps
    .map((s, i) => ({ id: s.id, depth: depthOf(s.id), after: after.get(s.id) ?? [], i }))
    .sort((a, b) => a.depth - b.depth || a.i - b.i)
    .map(({ id, depth, after }) => ({ id, depth, after }))
}
