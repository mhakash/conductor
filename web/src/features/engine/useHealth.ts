import { useQuery } from '@tanstack/react-query'

import { api } from '@/lib/api'

export type Health = {
  status: 'ok'
  version: string
  commit: string
  startedAt: string
}

export function useHealth() {
  return useQuery({
    queryKey: ['health'],
    queryFn: ({ signal }) => api.get<Health>('/health', { signal }),
    refetchInterval: 10_000,
    retry: false,
  })
}
