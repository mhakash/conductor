import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api } from '@/lib/api'

export type Position = { x: number; y: number }

export type Step = {
  id: string
  block: string
  name: string
  config: Record<string, unknown>
  position: Position
} & StepSettings

export type Retry = {
  /** Retries after the first attempt. */
  max: number
  /** Go duration before the first retry; default 10s. */
  delay?: string
  backoff?: 'fixed' | 'exponential'
}

export type ForEach = {
  /** Expression (without {{ }}) that evaluates to a list. */
  items: string
  /** Items at a time; default 1. */
  concurrency?: number
}

/** How a step runs, independent of its block. See ADR 0006. */
export type StepSettings = {
  /** Go duration such as "30s" or "10m"; empty means the engine default. */
  timeout?: string
  /** Expression (without {{ }}); the step is skipped unless it is true. */
  if?: string
  retry?: Retry
  continueOnError?: boolean
  forEach?: ForEach
}

export type Edge = { from: string; to: string }

export type InputType = 'text' | 'number' | 'boolean'

/** A value asked for when a run starts; read in expressions as inputs.<name>. */
export type WorkflowInput = {
  name: string
  label?: string
  type: InputType
  default?: string | number | boolean
  required?: boolean
  description?: string
}

export type Definition = {
  schemaVersion: number
  inputs?: WorkflowInput[]
  steps: Step[]
  edges: Edge[]
}

export type Workflow = {
  id: string
  name: string
  description: string
  definition: Definition
  createdAt: string
  updatedAt: string
}

export type WorkflowSummary = Omit<Workflow, 'definition'> & { stepCount: number }

export type WorkflowFields = Pick<Workflow, 'name' | 'description'> & {
  definition?: Definition
  /** The version being edited; the save is refused (409) if it is stale. */
  updatedAt?: string
}

const keys = {
  all: ['workflows'] as const,
  one: (id: string) => ['workflows', id] as const,
}

export function useWorkflows() {
  return useQuery({
    queryKey: keys.all,
    queryFn: ({ signal }) => api.get<WorkflowSummary[]>('/workflows', { signal }),
  })
}

export const fetchWorkflow = (id: string, signal?: AbortSignal) =>
  api.get<Workflow>(`/workflows/${id}`, { signal })

export function useWorkflow(id: string) {
  return useQuery({
    queryKey: keys.one(id),
    queryFn: ({ signal }) => fetchWorkflow(id, signal),
    // The editor owns the draft; refetching underneath it would be confusing.
    staleTime: Infinity,
  })
}

export function useCreateWorkflow() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: WorkflowFields) => api.post<Workflow>('/workflows', input),
    onSuccess: (wf) => {
      qc.setQueryData(keys.one(wf.id), wf)
      void qc.invalidateQueries({ queryKey: keys.all, exact: true })
    },
  })
}

export function useUpdateWorkflow(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: WorkflowFields) => api.put<Workflow>(`/workflows/${id}`, input),
    onSuccess: (wf) => {
      qc.setQueryData(keys.one(wf.id), wf)
      void qc.invalidateQueries({ queryKey: keys.all, exact: true })
    },
  })
}

export function useDeleteWorkflow() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/workflows/${id}`),
    onSuccess: (_, id) => {
      qc.removeQueries({ queryKey: keys.one(id) })
      void qc.invalidateQueries({ queryKey: keys.all, exact: true })
    },
  })
}
