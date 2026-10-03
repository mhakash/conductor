import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'

import { api } from '@/lib/api'

export type FieldType = 'text' | 'textarea' | 'code' | 'number' | 'boolean' | 'select' | 'keyvalue'

export type Field = {
  key: string
  label: string
  type: FieldType
  required?: boolean
  help?: string
  placeholder?: string
  default?: unknown
  options?: { value: string; label: string }[]
  language?: string
}

export type Block = {
  type: string
  name: string
  description: string
  category: string
  icon: string
  fields: Field[]
  outputs: { key: string; description: string }[]
}

export function useBlocks() {
  return useQuery({
    queryKey: ['blocks'],
    queryFn: ({ signal }) => api.get<Block[]>('/blocks', { signal }),
    staleTime: Infinity,
  })
}

/** Config for a freshly added step: every field that declares a default. */
export function defaultConfig(block: Block): Record<string, unknown> {
  return Object.fromEntries(
    block.fields.filter((f) => f.default !== undefined).map((f) => [f.key, f.default]),
  )
}

export function useBlockMap() {
  const { data } = useBlocks()
  return useMemo(() => new Map((data ?? []).map((b) => [b.type, b])), [data])
}

function isEmpty(v: unknown) {
  if (v === undefined || v === null) return true
  if (typeof v === 'string') return v.trim() === ''
  if (typeof v === 'object') return Object.keys(v).length === 0
  return false
}

/** Required fields that have no value yet. */
export function missingFields(block: Block, config: Record<string, unknown>): Field[] {
  return block.fields.filter((f) => f.required && isEmpty(config[f.key]))
}
