import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import type { Definition } from '@/features/workflows/api'
import { api } from '@/lib/api'

export type RunStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled' | 'skipped'

export const isActive = (s: RunStatus) => s === 'queued' || s === 'running'

export const statusLabel = (s: RunStatus) =>
  ({
    queued: 'Queued',
    running: 'Running',
    succeeded: 'Succeeded',
    failed: 'Failed',
    cancelled: 'Cancelled',
    skipped: 'Skipped',
  })[s]

export type StepRun = {
  stepId: string
  status: RunStatus
  error?: string
  outputs: Record<string, unknown>
  /** The config this step ran with: expressions evaluated, secrets masked. */
  config?: Record<string, unknown>
  startedAt: string | null
  finishedAt: string | null
}

export type Run = {
  id: string
  workflowId: string
  workflowName: string
  definition: Definition
  inputs?: Record<string, unknown>
  status: RunStatus
  error?: string
  createdAt: string
  startedAt: string | null
  finishedAt: string | null
  steps: StepRun[]
}

export type RunSummary = Omit<Run, 'definition' | 'steps' | 'inputs'> & { stepCount: number }

export type LogLine = {
  seq: number
  stepId: string
  stream: 'stdout' | 'stderr' | 'system'
  text: string
  time: string
}

// A single run streams live (see stream.ts); lists of runs are polled while
// any of them is active.
const LIST_POLL_MS = 2000

export const runKeys = {
  all: ['runs'] as const,
  lists: ['runs', 'list'] as const,
  list: (workflowId?: string) => ['runs', 'list', workflowId ?? 'all'] as const,
  one: (id: string) => ['runs', id] as const,
  logs: (id: string) => ['runs', id, 'logs'] as const,
}
const keys = runKeys

export function useRuns(workflowId?: string) {
  return useQuery({
    queryKey: keys.list(workflowId),
    queryFn: ({ signal }) =>
      api.get<RunSummary[]>(`/runs${workflowId ? `?workflowId=${workflowId}` : ''}`, { signal }),
    refetchInterval: (q) => (q.state.data?.some((r) => isActive(r.status)) ? LIST_POLL_MS : false),
  })
}

/** Loads a run once; useRunStream keeps it current while it is active. */
export function useRun(id: string | undefined) {
  return useQuery({
    queryKey: keys.one(id ?? ''),
    queryFn: ({ signal }) => api.get<Run>(`/runs/${id}`, { signal }),
    staleTime: Infinity,
    enabled: !!id,
  })
}

export function useRunLogs(id: string) {
  return useQuery({
    queryKey: keys.logs(id),
    queryFn: ({ signal }) => api.get<LogLine[]>(`/runs/${id}/logs`, { signal }),
    staleTime: Infinity,
  })
}

export function useStartRun() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({
      workflowId,
      inputs,
    }: {
      workflowId: string
      inputs?: Record<string, unknown>
    }) => api.post<Run>(`/workflows/${workflowId}/runs`, { inputs: inputs ?? {} }),
    onSuccess: (run) => {
      qc.setQueryData(keys.one(run.id), run)
      void qc.invalidateQueries({ queryKey: keys.all })
    },
  })
}

export function useCancelRun() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.post<void>(`/runs/${id}/cancel`, {}),
    onSettled: (_, __, id) => qc.invalidateQueries({ queryKey: keys.one(id) }),
  })
}
