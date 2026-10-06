import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo } from 'react'

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

/**
 * A value a workflow hands back when it succeeds, to a workflow that runs it.
 * `value` is an expression without {{ }}, evaluated after every step.
 */
export type WorkflowOutput = {
  name: string
  value: string
  description?: string
}

export type Definition = {
  schemaVersion: number
  inputs?: WorkflowInput[]
  outputs?: WorkflowOutput[]
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

export type WorkflowSummary = Omit<Workflow, 'definition'> & {
  stepCount: number
  /** What a step running this workflow gives it and gets back. */
  inputs: WorkflowInput[]
  outputs: WorkflowOutput[]
}

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

export function useWorkflowMap() {
  const { data } = useWorkflows()
  return useMemo(() => new Map((data ?? []).map((w) => [w.id, w])), [data])
}

// YAML (ADR 0009): the server converts, so the format lives in one place.

/** Renders a workflow, saved or not, as YAML. */
export const toYAML = (fields: WorkflowFields) => api.postText('/workflows/to-yaml', fields)

/** Parses YAML into workflow fields without saving anything. */
export const fromYAML = (yaml: string) =>
  api.post<Required<Pick<WorkflowFields, 'name' | 'description' | 'definition'>>>(
    '/workflows/from-yaml',
    yaml,
  )

/** Where a saved workflow downloads as a YAML file. */
export const exportURL = (id: string) => `/api/workflows/${id}/export`

export function useImportWorkflow() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (yaml: string) => api.post<Workflow>('/workflows/import', yaml),
    onSuccess: (wf) => {
      qc.setQueryData(keys.one(wf.id), wf)
      void qc.invalidateQueries({ queryKey: keys.all, exact: true })
    },
  })
}
