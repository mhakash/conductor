import type { Step } from '@/features/workflows/api'

import { statusLabel, type StepRun } from './api'

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
