import { describe, expect, it } from 'vitest'

import { executionOrder } from './order'

const step = (id: string) => ({
  id,
  block: 'shell',
  name: id,
  config: {},
  position: { x: 0, y: 0 },
})

describe('executionOrder', () => {
  it('orders by dependency depth, keeping definition order within a level', () => {
    const order = executionOrder({
      schemaVersion: 1,
      // Deliberately listed out of order.
      steps: [step('email'), step('jira'), step('tag'), step('confluence')],
      edges: [
        { from: 'tag', to: 'jira' },
        { from: 'tag', to: 'confluence' },
        { from: 'jira', to: 'email' },
        { from: 'confluence', to: 'email' },
      ],
    })
    expect(order.map((s) => `${s.id}@${s.depth}`)).toEqual([
      'tag@0',
      'jira@1',
      'confluence@1',
      'email@2',
    ])
    expect(order[3].after).toEqual(['jira', 'confluence'])
  })
})
