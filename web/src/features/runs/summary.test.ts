import { describe, expect, it } from 'vitest'

import type { Step } from '@/features/workflows/api'

import type { StepRun } from './api'
import { stepSummary } from './summary'

const def: Step = {
  id: 'poll',
  block: 'http',
  name: '',
  config: {},
  position: { x: 0, y: 0 },
  retry: { max: 3 },
}
const run = (s: Partial<StepRun>): StepRun => ({
  stepId: 'poll',
  status: 'running',
  outputs: {},
  startedAt: null,
  finishedAt: null,
  ...s,
})

describe('stepSummary', () => {
  it('shows progress while running', () => {
    expect(stepSummary(run({ attempt: 2 }), def)).toBe('attempt 2 of 4')
    expect(stepSummary(run({ items: 5, itemsDone: 2 }), undefined)).toBe('2 of 5 items')
    expect(stepSummary(run({ attempt: 1 }), def)).toBe('Running')
  })

  it('prefers the reason a step ended', () => {
    expect(stepSummary(run({ status: 'skipped', error: 'condition is false: x' }), def)).toBe(
      'condition is false: x',
    )
    expect(stepSummary(run({ status: 'succeeded', attempt: 3 }), def)).toBe(
      'succeeded on attempt 3 of 4',
    )
    expect(stepSummary(run({ status: 'waiting' }), undefined)).toBe('Waiting for approval')
  })
})
