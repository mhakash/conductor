import type { WorkflowInput } from '@/features/workflows/api'

export type InputValues = Record<string, string | boolean>

/** Form values for `inputs`, from `initial` where given, else their defaults. */
export function initialValues(
  inputs: WorkflowInput[],
  initial?: Record<string, unknown>,
): InputValues {
  return Object.fromEntries(
    inputs.map((i) => {
      const v = initial?.[i.name] ?? i.default
      if (i.type === 'boolean') return [i.name, v === true || v === 'true']
      return [i.name, v === undefined || v === null ? '' : String(v)]
    }),
  )
}

/** Values to send: blanks are left out, so the server applies defaults. */
export function filledValues(values: InputValues): Record<string, unknown> {
  return Object.fromEntries(Object.entries(values).filter(([, v]) => v !== ''))
}
