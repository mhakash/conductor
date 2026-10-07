import { FormField, Input, Select, Textarea } from '@/components/ui/Input'
import type { WorkflowInput } from '@/features/workflows/api'
import { cn } from '@/lib/cn'

import type { InputValues } from './inputValues'

/** Fields for a list of inputs: a run's inputs, or what an input step asks for. */
export function InputFields({
  inputs,
  values,
  onChange,
  idPrefix,
}: {
  inputs: WorkflowInput[]
  values: InputValues
  onChange: (name: string, value: string | boolean) => void
  idPrefix: string
}) {
  return inputs.map((i) => {
    const id = `${idPrefix}-${i.name}`
    const value = String(values[i.name] ?? '')
    if (i.type === 'boolean') {
      const on = values[i.name] === true
      return (
        <label key={i.name} htmlFor={id} className="flex items-center justify-between gap-3">
          <span>
            <span className="block text-xs font-medium text-fg-muted">{i.label || i.name}</span>
            {i.description && <span className="block text-xs text-fg-subtle">{i.description}</span>}
          </span>
          <button
            id={id}
            type="button"
            role="switch"
            aria-checked={on}
            onClick={() => onChange(i.name, !on)}
            className={cn(
              'relative h-5 w-9 shrink-0 rounded-full transition-colors',
              on ? 'bg-brass' : 'bg-line-strong',
            )}
          >
            <span
              className={cn(
                'absolute top-0.5 left-0.5 size-4 rounded-full bg-fg transition-transform',
                on && 'translate-x-4',
              )}
            />
          </button>
        </label>
      )
    }
    return (
      <FormField
        key={i.name}
        label={i.label || i.name}
        htmlFor={id}
        required={i.required}
        help={i.description}
      >
        {i.type === 'select' ? (
          <Select id={id} value={value} onChange={(e) => onChange(i.name, e.target.value)}>
            <option value="" disabled={i.required}>
              {i.required ? 'Choose…' : '—'}
            </option>
            {(i.options ?? []).map((o) => (
              <option key={o} value={o}>
                {o}
              </option>
            ))}
          </Select>
        ) : i.type === 'longtext' ? (
          <Textarea
            id={id}
            value={value}
            rows={4}
            placeholder={i.default !== undefined ? String(i.default) : undefined}
            onChange={(e) => onChange(i.name, e.target.value)}
          />
        ) : (
          <Input
            id={id}
            type={i.type === 'number' ? 'number' : 'text'}
            value={value}
            placeholder={i.default !== undefined ? String(i.default) : undefined}
            className={i.type === 'text' ? 'font-mono text-xs' : undefined}
            onChange={(e) => onChange(i.name, e.target.value)}
          />
        )}
      </FormField>
    )
  })
}
