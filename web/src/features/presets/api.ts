import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import type { Retry } from '@/features/workflows/api'
import { api } from '@/lib/api'

/**
 * A saved step configuration. Adding one to a workflow copies it; editing
 * the preset later doesn't change steps made from it.
 */
export type Preset = {
  id: string
  name: string
  description: string
  block: string
  step: PresetStep
  createdAt: string
  updatedAt: string
}

/** What a preset fills in. Conditions and loops refer to a workflow's own steps, so they're left out. */
export type PresetStep = {
  config: Record<string, unknown>
  timeout?: string
  retry?: Retry
  continueOnError?: boolean
}

export type PresetFields = Pick<Preset, 'name' | 'description' | 'block' | 'step'>

const keys = { all: ['presets'] as const }

export function usePresets() {
  return useQuery({
    queryKey: keys.all,
    queryFn: ({ signal }) => api.get<Preset[]>('/presets', { signal }),
  })
}

export function useSavePreset() {
  const qc = useQueryClient()
  return useMutation({
    /** Creates a preset, or replaces the one with `id`. */
    mutationFn: ({ id, ...fields }: PresetFields & { id?: string }) =>
      id ? api.put<Preset>(`/presets/${id}`, fields) : api.post<Preset>('/presets', fields),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.all }),
  })
}

export function useDeletePreset() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/presets/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.all }),
  })
}
