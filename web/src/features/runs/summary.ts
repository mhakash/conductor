import type { Step } from '@/features/workflows/api'

import { isActive, statusLabel, type RunSummary, type StepRun } from './api'

/** What a step card says under its name: progress while running, else why it ended. */
export function stepSummary(step: StepRun, def: Step | undefined): string {
  const loop = step.items ? `${step.itemsDone ?? 0} of ${step.items} items` : ''
  const tries =
    def?.retry && step.attempt && step.attempt > 1
      ? `attempt ${step.attempt} of ${def.retry.max + 1}`
      : ''
  if (step.status === 'running') {
    return [loop, tries].filter(Boolean).join(' · ') || statusLabel(step.status)
  }
  if (step.error) return step.error
  if (step.status === 'succeeded' && (loop || tries)) {
    return loop ? `${step.items} items` : `succeeded on ${tries}`
  }
  return statusLabel(step.status)
}

/**
 * Shows a step that runs a workflow as waiting while every run it has going
 * is waiting for approval. (The engine records such a step as running.)
 */
export function withChildWaits(steps: StepRun[], children: RunSummary[]): StepRun[] {
  const going = Map.groupBy(
    children.filter((c) => c.parent && isActive(c.status)),
    (c) => c.parent!.stepId,
  )
  let changed = false
  const out = steps.map((s) => {
    const runs = going.get(s.stepId)
    if (s.status !== 'running' || !runs?.every((c) => c.status === 'waiting')) return s
    changed = true
    return { ...s, status: 'waiting' as const }
  })
  return changed ? out : steps
}
