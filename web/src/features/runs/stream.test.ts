import { QueryClient } from '@tanstack/react-query'
import { describe, expect, it } from 'vitest'

import { runKeys, type LogLine, type Run } from './api'
import { applyEvent, mergeLines } from './stream'

const line = (seq: number): LogLine => ({
  seq,
  stepId: 'a',
  stream: 'stdout',
  text: `line ${seq}`,
  time: '2026-01-01T00:00:00Z',
})

const run = (): Run => ({
  id: 'r1',
  workflowId: 'w',
  workflowName: 'W',
  definition: { schemaVersion: 1, steps: [], edges: [] },
  status: 'running',
  createdAt: '2026-01-01T00:00:00Z',
  startedAt: '2026-01-01T00:00:00Z',
  finishedAt: null,
  steps: [
    { stepId: 'a', status: 'running', outputs: {}, startedAt: null, finishedAt: null },
    { stepId: 'b', status: 'queued', outputs: {}, startedAt: null, finishedAt: null },
  ],
  children: [],
})

describe('mergeLines', () => {
  it('appends only lines newer than the last seq', () => {
    const have = [line(1), line(2)]
    expect(mergeLines(have, [line(2), line(3)]).map((l) => l.seq)).toEqual([1, 2, 3])
    expect(mergeLines(have, [line(1)])).toBe(have)
  })
})

describe('applyEvent', () => {
  it('applies snapshot, step, logs and run events to the cache', () => {
    const qc = new QueryClient()
    applyEvent(qc, 'r1', { type: 'snapshot', run: run(), lines: [line(1)] })
    applyEvent(qc, 'r1', {
      type: 'step',
      step: {
        stepId: 'a',
        status: 'succeeded',
        outputs: { x: 1 },
        startedAt: null,
        finishedAt: null,
      },
    })
    applyEvent(qc, 'r1', { type: 'logs', lines: [line(1), line(2)] })
    applyEvent(qc, 'r1', {
      type: 'run',
      state: { status: 'succeeded', startedAt: null, finishedAt: '2026-01-01T00:00:05Z' },
    })

    const r = qc.getQueryData<Run>(runKeys.one('r1'))!
    expect(r.status).toBe('succeeded')
    expect(r.steps.map((s) => s.status)).toEqual(['succeeded', 'queued'])
    expect(qc.getQueryData<LogLine[]>(runKeys.logs('r1'))!.map((l) => l.seq)).toEqual([1, 2])
  })

  it('adds a child run, then updates it in place', () => {
    const qc = new QueryClient()
    applyEvent(qc, 'r1', { type: 'snapshot', run: run() })
    const child = {
      id: 'c1',
      workflowId: 'w2',
      workflowName: 'Release',
      status: 'running' as const,
      stepCount: 2,
      createdAt: '2026-01-01T00:00:01Z',
      startedAt: '2026-01-01T00:00:01Z',
      finishedAt: null,
      parent: { runId: 'r1', workflowName: 'W', stepId: 'a' },
    }
    applyEvent(qc, 'r1', { type: 'child', child })
    applyEvent(qc, 'r1', { type: 'child', child: { ...child, status: 'waiting' } })

    const r = qc.getQueryData<Run>(runKeys.one('r1'))!
    expect(r.children.map((c) => c.status)).toEqual(['waiting'])
  })
})
