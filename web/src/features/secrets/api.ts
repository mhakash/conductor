import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api } from '@/lib/api'

/** A stored secret. Values are write-only: the API never returns them. */
export type Secret = { name: string; createdAt: string; updatedAt: string }

export const SECRET_NAME = /^[A-Z][A-Z0-9_]{0,63}$/

const key = ['secrets'] as const

export function useSecrets() {
  return useQuery({
    queryKey: key,
    queryFn: ({ signal }) => api.get<Secret[]>('/secrets', { signal }),
  })
}

export function useSetSecret() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ name, value }: { name: string; value: string }) =>
      api.put<Secret>(`/secrets/${name}`, { value }),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }),
  })
}

export function useDeleteSecret() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => api.delete(`/secrets/${name}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }),
  })
}
