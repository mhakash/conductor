import { useQueryClient, type QueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'

import { runKeys, type LogLine, type Run, type StepRun } from './api'

type RunState = Pick<Run, 'status' | 'error' | 'startedAt' | 'finishedAt'>

export type RunEvent =
  | { type: 'snapshot'; run: Run; lines?: LogLine[] }
  | { type: 'run'; state: RunState }
  | { type: 'step'; step: StepRun }
  | { type: 'logs'; lines: LogLine[] }
  | { type: 'end' }

/** Appends lines newer than what we have; events may overlap after a reconnect. */
export function mergeLines(have: LogLine[], incoming: LogLine[]): LogLine[] {
  const last = have.at(-1)?.seq ?? 0
  const fresh = incoming.filter((l) => l.seq > last)
  return fresh.length ? [...have, ...fresh] : have
}

export function applyEvent(qc: QueryClient, id: string, e: RunEvent) {
  switch (e.type) {
    case 'snapshot':
      qc.setQueryData<Run>(runKeys.one(id), e.run)
      qc.setQueryData<LogLine[]>(runKeys.logs(id), e.lines ?? [])
      break
    case 'run':
      qc.setQueryData<Run>(runKeys.one(id), (r) => r && { ...r, ...e.state })
      break
    case 'step':
      qc.setQueryData<Run>(
        runKeys.one(id),
        (r) =>
          r && {
            ...r,
            steps: r.steps.map((s) => (s.stepId === e.step.stepId ? e.step : s)),
          },
      )
      break
    case 'logs':
      qc.setQueryData<LogLine[]>(runKeys.logs(id), (have = []) => mergeLines(have, e.lines))
      break
  }
}

/**
 * Keeps the cached run and its logs live while `active`. The server sends a
 * snapshot first, so (re)connecting never loses anything.
 */
export function useRunStream(id: string, active: boolean): { live: boolean } {
  const qc = useQueryClient()
  const [live, setLive] = useState(false)

  useEffect(() => {
    if (!active) return
    const source = new EventSource(`/api/runs/${id}/events`)
    source.onopen = () => setLive(true)
    // EventSource reconnects by itself; just reflect the gap.
    source.onerror = () => setLive(false)
    source.onmessage = (m) => {
      const e = JSON.parse(m.data as string) as RunEvent
      if (e.type === 'end') {
        source.close()
        setLive(false)
        void qc.invalidateQueries({ queryKey: runKeys.lists })
        return
      }
      applyEvent(qc, id, e)
    }
    return () => {
      source.close()
      setLive(false)
    }
  }, [id, active, qc])

  return { live }
}
