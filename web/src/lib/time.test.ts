import { describe, expect, it } from 'vitest'

import { elapsed, formatDuration } from './time'

describe('formatDuration', () => {
  it.each([
    [850, '850ms'],
    [4200, '4.2s'],
    [185_000, '3m 05s'],
    [3_720_000, '1h 02m'],
  ])('%i → %s', (ms, want) => expect(formatDuration(ms)).toBe(want))
})

describe('elapsed', () => {
  it('measures to now when unfinished', () => {
    expect(elapsed('2026-01-01T00:00:00Z', null, Date.parse('2026-01-01T00:00:02Z'))).toBe(2000)
    expect(elapsed(null, null)).toBeNull()
  })
})
