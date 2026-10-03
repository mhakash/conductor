import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

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
  startedAt: string | null
  finishedAt: string | null
}

export type Run = {
  id: string
  workflowId: string
  workflowName: string
  definition: Definition
  status: RunStatus
  error?: string
  createdAt: string
  startedAt: string | null
  finishedAt: string | null
  steps: StepRun[]
}

export type RunSummary = Omit<Run, 'definition' | 'steps'> & { stepCount: number }

export type LogLine = {
  seq: number
  stepId: string
  stream: 'stdout' | 'stderr' | 'system'
  text: string
  time: string
}

// Until runs stream live (phase 3), active runs are polled.
const POLL_MS = 1000

const keys = {
  all: ['runs'] as const,
  list: (workflowId?: string) => ['runs', 'list', workflowId ?? 'all'] as const,
  one: (id: string) => ['runs', id] as const,
  logs: (id: string) => ['runs', id, 'logs'] as const,
}

export function useRuns(workflowId?: string) {
  return useQuery({
    queryKey: keys.list(workflowId),
    queryFn: ({ signal }) =>
      api.get<RunSummary[]>(`/runs${workflowId ? `?workflowId=${workflowId}` : ''}`, { signal }),
    refetchInterval: (q) => (q.state.data?.some((r) => isActive(r.status)) ? POLL_MS : false),
  })
}

export function useRun(id: string) {
  return useQuery({
    queryKey: keys.one(id),
    queryFn: ({ signal }) => api.get<Run>(`/runs/${id}`, { signal }),
    refetchInterval: (q) => (q.state.data && isActive(q.state.data.status) ? POLL_MS : false),
  })
}

export function useRunLogs(id: string, active: boolean) {
  return useQuery({
    queryKey: keys.logs(id),
    queryFn: ({ signal }) => api.get<LogLine[]>(`/runs/${id}/logs`, { signal }),
    refetchInterval: active ? POLL_MS : false,
    placeholderData: keepPreviousData,
  })
}

export function useStartRun() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (workflowId: string) => api.post<Run>(`/workflows/${workflowId}/runs`, {}),
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
