import {
  addEdge,
  applyEdgeChanges,
  applyNodeChanges,
  type Connection,
  type Edge,
  type EdgeChange,
  type NodeChange,
  type XYPosition,
} from '@xyflow/react'
import { create } from 'zustand'

import { defaultConfig, type Block } from '@/features/blocks/api'
import type { Workflow, WorkflowInput } from '@/features/workflows/api'
import type { Problem } from '@/lib/api'

import {
  edgeId,
  nextStepId,
  STEP_ID_PATTERN,
  toFlow,
  wouldCreateCycle,
  type StepData,
  type StepNode,
} from './graph'

type EditorState = {
  workflowId: string | null
  /** updatedAt of the version this draft is based on, for conflict checks. */
  version: string | null
  name: string
  description: string
  inputs: WorkflowInput[]
  nodes: StepNode[]
  edges: Edge[]
  /** True when the draft differs from what was last loaded or saved. */
  dirty: boolean
  /** Problems from the last failed save or run, cleared by the next successful save. */
  problems: Problem[]

  load: (wf: Workflow) => void
  markSaved: (version: string) => void
  setProblems: (problems: Problem[]) => void

  setName: (name: string) => void
  setDescription: (description: string) => void
  setInputs: (inputs: WorkflowInput[]) => void
  onNodesChange: (changes: NodeChange<StepNode>[]) => void
  onEdgesChange: (changes: EdgeChange[]) => void
  connect: (c: Connection) => void
  /** Adds a selected step; with `after`, also makes it depend on that step. */
  addStep: (block: Block, position: XYPosition, after?: string) => string
  updateStep: (id: string, patch: Partial<StepData>) => void
  /** Returns an error message, or null when the rename was applied. */
  renameStep: (from: string, to: string) => string | null
  removeStep: (id: string) => void
  setPositions: (positions: Map<string, XYPosition>) => void
}

// Selection and measurement are view state; they shouldn't mark the draft dirty.
const isViewOnly = (c: NodeChange | EdgeChange) => c.type === 'select' || c.type === 'dimensions'

export const useEditor = create<EditorState>()((set, get) => ({
  workflowId: null,
  version: null,
  name: '',
  description: '',
  inputs: [],
  nodes: [],
  edges: [],
  dirty: false,
  problems: [],

  load: (wf) => {
    const { nodes, edges } = toFlow(wf.definition)
    set({
      workflowId: wf.id,
      version: wf.updatedAt,
      name: wf.name,
      description: wf.description,
      inputs: wf.definition.inputs ?? [],
      nodes,
      edges,
      dirty: false,
      problems: [],
    })
  },
  markSaved: (version) => set({ version, dirty: false, problems: [] }),
  setProblems: (problems) => set({ problems }),

  setName: (name) => set({ name, dirty: true }),
  setDescription: (description) => set({ description, dirty: true }),
  setInputs: (inputs) => set({ inputs, dirty: true }),

  onNodesChange: (changes) =>
    set((s) => ({
      nodes: applyNodeChanges(changes, s.nodes),
      dirty: s.dirty || changes.some((c) => !isViewOnly(c)),
    })),

  onEdgesChange: (changes) =>
    set((s) => ({
      edges: applyEdgeChanges(changes, s.edges),
      dirty: s.dirty || changes.some((c) => !isViewOnly(c)),
    })),

  connect: (c) => {
    const { edges } = get()
    if (wouldCreateCycle(edges, c.source, c.target)) return
    const id = edgeId({ from: c.source, to: c.target })
    if (edges.some((e) => e.id === id)) return
    set({ edges: addEdge({ ...c, id }, edges), dirty: true })
  },

  addStep: (block, position, after) => {
    const { nodes, edges } = get()
    const id = nextStepId(block.type, new Set(nodes.map((n) => n.id)))
    const node: StepNode = {
      id,
      type: 'step',
      position,
      selected: true,
      data: { block: block.type, name: block.name, config: defaultConfig(block) },
    }
    set({
      nodes: [...nodes.map((n) => (n.selected ? { ...n, selected: false } : n)), node],
      edges: after
        ? [...edges, { id: edgeId({ from: after, to: id }), source: after, target: id }]
        : edges,
      dirty: true,
    })
    return id
  },

  updateStep: (id, patch) =>
    set((s) => ({
      nodes: s.nodes.map((n) => (n.id === id ? { ...n, data: { ...n.data, ...patch } } : n)),
      dirty: true,
    })),

  renameStep: (from, to) => {
    if (from === to) return null
    if (!STEP_ID_PATTERN.test(to)) {
      return 'Start with a letter; use only a–z, 0–9 and _.'
    }
    const { nodes, edges } = get()
    if (nodes.some((n) => n.id === to)) return `Another step already uses “${to}”.`

    const rename = (id: string) => (id === from ? to : id)
    set({
      nodes: nodes.map((n) => (n.id === from ? { ...n, id: to } : n)),
      edges: edges.map((e) => {
        const source = rename(e.source)
        const target = rename(e.target)
        return { ...e, source, target, id: edgeId({ from: source, to: target }) }
      }),
      dirty: true,
    })
    return null
  },

  removeStep: (id) =>
    set((s) => ({
      nodes: s.nodes.filter((n) => n.id !== id),
      edges: s.edges.filter((e) => e.source !== id && e.target !== id),
      dirty: true,
    })),

  setPositions: (positions) =>
    set((s) => ({
      nodes: s.nodes.map((n) => {
        const p = positions.get(n.id)
        return p ? { ...n, position: p } : n
      }),
      dirty: true,
    })),
}))
