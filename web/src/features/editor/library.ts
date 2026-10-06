import { useMemo } from 'react'

import { CALL_BLOCK, defaultConfig, useBlocks, type Block } from '@/features/blocks/api'
import { usePresets, type Preset } from '@/features/presets/api'
import { useWorkflows, type StepSettings, type WorkflowSummary } from '@/features/workflows/api'

/** What a new step starts with. */
export type StepTemplate = {
  block: string
  name: string
  /** Base for the new step's id, e.g. the block type or the preset's name. */
  idBase: string
  config: Record<string, unknown>
} & StepSettings

/** Something that can be added to a workflow as a step. */
export type LibraryItem = {
  /** Unique across kinds, e.g. "block:shell" or "preset:<id>". */
  key: string
  kind: 'block' | 'preset' | 'workflow'
  name: string
  description: string
  icon: string
  /** The section it is listed under. */
  group: string
  template: StepTemplate
}

export function blockItem(b: Block): LibraryItem {
  return {
    key: `block:${b.type}`,
    kind: 'block',
    name: b.name,
    description: b.description,
    icon: b.icon,
    group: b.category,
    template: { block: b.type, name: b.name, idBase: b.type, config: defaultConfig(b) },
  }
}

export function presetItem(p: Preset, block: Block | undefined): LibraryItem {
  const { config, ...settings } = structuredClone(p.step)
  return {
    key: `preset:${p.id}`,
    kind: 'preset',
    name: p.name,
    description: p.description || block?.name || p.block,
    icon: block?.icon ?? '',
    group: 'Presets',
    template: { block: p.block, name: p.name, idBase: p.name, config, ...settings },
  }
}

export function workflowItem(wf: WorkflowSummary): LibraryItem {
  return {
    key: `workflow:${wf.id}`,
    kind: 'workflow',
    name: wf.name,
    description: wf.description || describeWorkflow(wf),
    icon: 'workflow',
    group: 'Workflows',
    template: {
      block: CALL_BLOCK,
      name: wf.name,
      idBase: wf.name,
      config: { workflow: wf.id, inputs: {} },
    },
  }
}

function describeWorkflow(wf: WorkflowSummary) {
  const parts = [`${wf.stepCount} ${wf.stepCount === 1 ? 'step' : 'steps'}`]
  if (wf.inputs.length)
    parts.push(`${wf.inputs.length} ${wf.inputs.length === 1 ? 'input' : 'inputs'}`)
  if (wf.outputs.length) {
    parts.push(`${wf.outputs.length} ${wf.outputs.length === 1 ? 'output' : 'outputs'}`)
  }
  return parts.join(' · ')
}

/**
 * Everything the workflow being edited can use: blocks, presets and the
 * other workflows (run as a step).
 */
export function useLibrary(workflowId: string): LibraryItem[] {
  const blocks = useBlocks().data
  const presets = usePresets().data
  const workflows = useWorkflows().data
  return useMemo(() => {
    const byType = new Map((blocks ?? []).map((b) => [b.type, b]))
    return [
      ...(blocks ?? []).map(blockItem),
      ...(presets ?? []).map((p) => presetItem(p, byType.get(p.block))),
      ...(workflows ?? []).filter((w) => w.id !== workflowId).map(workflowItem),
    ]
  }, [blocks, presets, workflows, workflowId])
}

/** Case-insensitive match on name and description, every word anywhere. */
export function matches(item: LibraryItem, query: string): boolean {
  const text = `${item.name} ${item.description}`.toLowerCase()
  return query
    .toLowerCase()
    .split(/\s+/)
    .every((word) => text.includes(word))
}
