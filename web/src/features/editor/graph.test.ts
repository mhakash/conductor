import type { Edge } from '@xyflow/react'
import { describe, expect, it } from 'vitest'

import {
  ancestorsOf,
  findFreeSpot,
  nextStepId,
  toDefinition,
  toFlow,
  wouldCreateCycle,
} from './graph'

const edges = (...pairs: [string, string][]): Edge[] =>
  pairs.map(([source, target]) => ({ id: `${source}->${target}`, source, target }))

describe('wouldCreateCycle', () => {
  const chain = edges(['tag', 'push'], ['push', 'pipeline'])

  it('rejects self edges', () => {
    expect(wouldCreateCycle([], 'tag', 'tag')).toBe(true)
  })

  it('rejects edges that close a loop', () => {
    expect(wouldCreateCycle(chain, 'pipeline', 'tag')).toBe(true)
  })

  it('allows forward and parallel edges', () => {
    expect(wouldCreateCycle(chain, 'tag', 'pipeline')).toBe(false)
    expect(wouldCreateCycle(chain, 'pipeline', 'jira')).toBe(false)
  })
})

describe('nextStepId', () => {
  it('uses the base when free, then numbers it', () => {
    expect(nextStepId('shell', new Set())).toBe('shell')
    expect(nextStepId('shell', new Set(['shell', 'shell_2']))).toBe('shell_3')
  })

  it('turns any base into a valid identifier', () => {
    expect(nextStepId('3-HTTP call', new Set())).toBe('http_call')
    expect(nextStepId('---', new Set())).toBe('step')
  })
})

describe('toFlow / toDefinition', () => {
  it('round-trips a definition, rounding positions', () => {
    const def = {
      schemaVersion: 1,
      steps: [
        {
          id: 'tag',
          block: 'shell',
          name: 'Tag',
          config: { command: 'git tag' },
          position: { x: 1, y: 2 },
        },
        { id: 'push', block: 'shell', name: 'Push', config: {}, position: { x: 300, y: 2 } },
      ],
      edges: [{ from: 'tag', to: 'push' }],
    }
    const { nodes, edges } = toFlow(def)
    nodes[0].position = { x: 10.4, y: 20.6 }
    const out = toDefinition(nodes, edges)
    expect(out.steps[0].position).toEqual({ x: 10, y: 21 })
    expect(out.steps[0].config).toEqual({ command: 'git tag' })
    expect(out.edges).toEqual(def.edges)
  })
})

describe('findFreeSpot', () => {
  const node = (id: string, x: number, y: number) =>
    ({
      id,
      type: 'step',
      position: { x, y },
      data: { block: 'shell', name: '', config: {} },
    }) as const

  it('keeps a free position', () => {
    expect(findFreeSpot([node('a', 0, 0)], { x: 400, y: 0 })).toEqual({ x: 400, y: 0 })
  })

  it('moves down past occupied spots', () => {
    const nodes = [node('a', 0, 0), node('b', 0, 104)]
    expect(findFreeSpot(nodes, { x: 0, y: 0 })).toEqual({ x: 0, y: 208 })
  })
})

describe('timeout', () => {
  it('round-trips a step timeout and drops blank ones', () => {
    const { nodes, edges } = toFlow({
      schemaVersion: 1,
      steps: [
        { id: 'a', block: 'shell', name: '', config: {}, position: { x: 0, y: 0 }, timeout: '5m' },
        { id: 'b', block: 'shell', name: '', config: {}, position: { x: 0, y: 0 } },
      ],
      edges: [],
    })
    nodes[1].data.timeout = '  '
    const out = toDefinition(nodes, edges)
    expect(out.steps[0].timeout).toBe('5m')
    expect('timeout' in out.steps[1]).toBe(false)
  })
})

describe('ancestorsOf', () => {
  it('collects transitive dependencies only', () => {
    const e = edges(['tag', 'push'], ['push', 'jira'], ['push', 'confluence'], ['jira', 'email'])
    expect([...ancestorsOf('email', e)].sort()).toEqual(['jira', 'push', 'tag'])
    expect(ancestorsOf('tag', e).size).toBe(0)
  })
})
