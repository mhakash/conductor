import { createContext, useContext, useMemo } from 'react'

import { isActive, useRun, useRuns, type RunStatus } from '@/features/runs/api'
import { useRunStream } from '@/features/runs/stream'

/** Step statuses from the workflow's most recent run, keyed by step id. */
export const RunOverlayContext = createContext<Map<string, RunStatus>>(new Map())

export function useStepRunStatus(stepId: string): RunStatus | undefined {
  return useContext(RunOverlayContext).get(stepId)
}

/** Loads the latest run of a workflow and keeps it live while it runs. */
export function useLatestRun(workflowId: string) {
  const latestId = useRuns(workflowId).data?.[0]?.id
  const run = useRun(latestId).data
  useRunStream(latestId ?? '', !!run && isActive(run.status))

  const statuses = useMemo(
    () => new Map((run?.steps ?? []).map((s) => [s.stepId, s.status])),
    [run?.steps],
  )
  return { run, statuses }
}
