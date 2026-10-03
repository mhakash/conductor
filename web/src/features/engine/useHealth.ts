import { useQuery } from '@tanstack/react-query'

import { apiGet } from '@/lib/api'

export type Health = {
  status: 'ok'
  version: string
  commit: string
  startedAt: string
}

export function useHealth() {
  return useQuery({
    queryKey: ['health'],
    queryFn: ({ signal }) => apiGet<Health>('/health', { signal }),
    refetchInterval: 10_000,
    retry: false,
  })
}
