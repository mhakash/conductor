import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api } from '@/lib/api'

export type Position = { x: number; y: number }

export type Step = {
  id: string
  block: string
  name: string
  config: Record<string, unknown>
  position: Position
}

export type Edge = { from: string; to: string }

export type Definition = {
  schemaVersion: number
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

export type WorkflowInput = Pick<Workflow, 'name' | 'description'> & { definition?: Definition }

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

export function useWorkflow(id: string) {
  return useQuery({
    queryKey: keys.one(id),
    queryFn: ({ signal }) => api.get<Workflow>(`/workflows/${id}`, { signal }),
    // The editor owns the draft; refetching underneath it would be confusing.
    staleTime: Infinity,
  })
}

export function useCreateWorkflow() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: WorkflowInput) => api.post<Workflow>('/workflows', input),
    onSuccess: (wf) => {
      qc.setQueryData(keys.one(wf.id), wf)
      void qc.invalidateQueries({ queryKey: keys.all, exact: true })
    },
  })
}

export function useUpdateWorkflow(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: WorkflowInput) => api.put<Workflow>(`/workflows/${id}`, input),
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
