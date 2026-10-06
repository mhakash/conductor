import type { Definition, Workflow } from '@/features/workflows/api'

import { SCHEMA_VERSION } from './graph'

/**
 * The scratch workflow: edited and run like any other, but never saved to
 * the server. It lives in this browser's storage so it survives leaving the
 * page, along with its latest test run.
 */
export const SCRATCH_ID = 'scratch'

const KEY = 'conductor.scratch'

type Stored = {
  name: string
  description: string
  definition: Definition
  runId?: string
}

function read(): Partial<Stored> {
  try {
    return JSON.parse(localStorage.getItem(KEY) ?? '{}') as Partial<Stored>
  } catch {
    return {}
  }
}

function write(patch: Partial<Stored>) {
  try {
    localStorage.setItem(KEY, JSON.stringify({ ...read(), ...patch }))
  } catch {
    // Storage may be unavailable (private mode); the scratch just won't persist.
  }
}

export function loadScratch(): { workflow: Workflow; runId?: string } {
  const s = read()
  return {
    workflow: {
      id: SCRATCH_ID,
      name: s.name ?? 'Scratch',
      description: s.description ?? '',
      definition: s.definition ?? {
        schemaVersion: SCHEMA_VERSION,
        inputs: [],
        steps: [],
        edges: [],
      },
      createdAt: '',
      updatedAt: '',
    },
    runId: s.runId,
  }
}

export const saveScratch = (fields: Omit<Stored, 'runId'>) => write(fields)
export const saveScratchRun = (runId: string | undefined) => write({ runId })

export function clearScratch() {
  try {
    localStorage.removeItem(KEY)
  } catch {
    // Nothing stored to clear.
  }
}
