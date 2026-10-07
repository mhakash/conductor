import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'

import type { WorkflowInput } from '@/features/workflows/api'
import { api } from '@/lib/api'

export type FieldType =
  | 'text'
  | 'textarea'
  | 'code'
  | 'number'
  | 'boolean'
  | 'select'
  | 'keyvalue'
  /** A saved workflow, by id. */
  | 'workflow'
  /** Values for the inputs of the workflow chosen in the block's `workflow` field. */
  | 'inputs'
  /** Fields to ask for, defined like workflow inputs (input block). */
  | 'form'

/** The block of a step that runs another workflow (ADR 0007). */
export const CALL_BLOCK = 'workflow'

/** Config of a CALL_BLOCK step. */
export type CallConfig = { workflow?: string; inputs?: Record<string, string> }

/** The block of a step that waits for someone to fill in a form (ADR 0006). */
export const INPUT_BLOCK = 'input'

/** Config of an INPUT_BLOCK step. */
export type FormConfig = { message?: string; fields?: WorkflowInput[] }

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
  /** Another field whose value is this code field's language. */
  languageFrom?: string
  /** Placeholder per value of the languageFrom field. */
  placeholders?: Record<string, string>
  /** Taken as written: `{{` in it is not an expression. */
  literal?: boolean
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
