import type { Edge as FlowEdge, Node, XYPosition } from '@xyflow/react'

import type {
  Definition,
  Edge,
  Step,
  StepSettings,
  WorkflowInput,
  WorkflowOutput,
} from '@/features/workflows/api'

export const SCHEMA_VERSION = 1

export type StepData = {
  block: string
  name: string
  config: Record<string, unknown>
} & StepSettings

export type StepNode = Node<StepData, 'step'>

export const STEP_ID_PATTERN = /^[a-z][a-z0-9_]{0,63}$/

export function edgeId(e: Edge) {
  return `${e.from}->${e.to}`
}

export function toFlow(def: Definition): { nodes: StepNode[]; edges: FlowEdge[] } {
  return {
    nodes: def.steps.map((s) => ({
      id: s.id,
      type: 'step',
      position: s.position,
      data: {
        block: s.block,
        name: s.name,
        config: s.config ?? {},
        timeout: s.timeout,
        if: s.if,
        retry: s.retry,
        continueOnError: s.continueOnError,
        forEach: s.forEach,
      },
    })),
    edges: def.edges.map((e) => ({ id: edgeId(e), source: e.from, target: e.to })),
  }
}

export function toDefinition(
  nodes: StepNode[],
  edges: FlowEdge[],
  inputs: WorkflowInput[] = [],
  outputs: WorkflowOutput[] = [],
): Definition {
  const steps: Step[] = nodes.map((n) => ({
    id: n.id,
    block: n.data.block,
    name: n.data.name,
    config: n.data.config,
    position: { x: Math.round(n.position.x), y: Math.round(n.position.y) },
    ...cleanSettings(n.data),
  }))
  return {
    schemaVersion: SCHEMA_VERSION,
    inputs,
    outputs,
    steps,
    edges: edges.map((e) => ({ from: e.source, to: e.target })),
  }
}

/** Drops settings left empty in the inspector, so they don't reach the definition. */
export function cleanSettings(d: StepSettings): StepSettings {
  const out: StepSettings = {}
  if (d.timeout?.trim()) out.timeout = d.timeout.trim()
  if (d.if?.trim()) out.if = d.if.trim()
  if (d.retry && d.retry.max > 0) {
    out.retry = { max: d.retry.max }
    if (d.retry.delay?.trim()) out.retry.delay = d.retry.delay.trim()
    if (d.retry.backoff === 'exponential') out.retry.backoff = 'exponential'
  }
  if (d.continueOnError) out.continueOnError = true
  if (d.forEach) {
    out.forEach = { items: d.forEach.items.trim() }
    if (d.forEach.concurrency && d.forEach.concurrency > 1) {
      out.forEach.concurrency = d.forEach.concurrency
    }
  }
  return out
}

/** True if adding source → target would close a loop (or is a self-edge). */
export function wouldCreateCycle(edges: FlowEdge[], source: string, target: string): boolean {
  if (source === target) return true
  // A cycle appears if source is already reachable from target.
  const next = new Map<string, string[]>()
  for (const e of edges) next.set(e.source, [...(next.get(e.source) ?? []), e.target])
  const stack = [target]
  const seen = new Set<string>()
  while (stack.length) {
    const id = stack.pop()!
    if (id === source) return true
    if (seen.has(id)) continue
    seen.add(id)
    stack.push(...(next.get(id) ?? []))
  }
  return false
}

/** First free id of the form `<base>`, `<base>_2`, `<base>_3`, … */
export function nextStepId(base: string, taken: Set<string>): string {
  const clean =
    base
      .toLowerCase()
      .replace(/[^a-z0-9_]/g, '_')
      .replace(/^[^a-z]+/, '') || 'step'
  if (!taken.has(clean)) return clean
  for (let i = 2; ; i++) {
    const id = `${clean}_${i}`
    if (!taken.has(id)) return id
  }
}

export const NODE_SIZE = { width: 240, height: 80 }
const GAP = 24

/**
 * Returns `desired`, or the nearest spot below it where a new node would not
 * overlap an existing one.
 */
export function findFreeSpot(nodes: StepNode[], desired: XYPosition): XYPosition {
  const overlaps = (p: XYPosition) =>
    nodes.some((n) => {
      const w = n.measured?.width ?? NODE_SIZE.width
      const h = n.measured?.height ?? NODE_SIZE.height
      return (
        p.x < n.position.x + w + GAP &&
        p.x + NODE_SIZE.width + GAP > n.position.x &&
        p.y < n.position.y + h + GAP &&
        p.y + NODE_SIZE.height + GAP > n.position.y
      )
    })
  const p = { ...desired }
  while (overlaps(p)) p.y += NODE_SIZE.height + GAP
  return p
}

/** Every step that finishes before `id` starts: its transitive dependencies. */
export function ancestorsOf(id: string, edges: FlowEdge[]): Set<string> {
  const parents = new Map<string, string[]>()
  for (const e of edges) parents.set(e.target, [...(parents.get(e.target) ?? []), e.source])
  const seen = new Set<string>()
  const stack = [...(parents.get(id) ?? [])]
  while (stack.length) {
    const p = stack.pop()!
    if (seen.has(p)) continue
    seen.add(p)
    stack.push(...(parents.get(p) ?? []))
  }
  return seen
}
