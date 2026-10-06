import { FormField, Input, Select, Textarea } from '@/components/ui/Input'
import type { Field } from '@/features/blocks/api'
import { cn } from '@/lib/cn'

import { CodeField } from './CodeField'
import { KeyValueField } from './KeyValueField'
import { CallInputsField, WorkflowField } from './WorkflowFields'

type ConfigFieldProps = {
  field: Field
  value: unknown
  onChange: (value: unknown) => void
  /** The step's whole config, for fields that depend on another (inputs on workflow). */
  config?: Record<string, unknown>
}

const str = (v: unknown) => (typeof v === 'string' ? v : v == null ? '' : String(v))

export function ConfigField({ field, value, onChange, config }: ConfigFieldProps) {
  const id = `field-${field.key}`

  if (field.type === 'boolean') {
    const on = value === true
    return (
      <div className="flex items-center justify-between gap-3">
        <label htmlFor={id} className="text-xs font-medium text-fg-muted">
          {field.label}
        </label>
        <button
          id={id}
          role="switch"
          aria-checked={on}
          onClick={() => onChange(!on)}
          className={cn(
            'relative h-5 w-9 rounded-full transition-colors',
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
      </div>
    )
  }

  return (
    <FormField label={field.label} htmlFor={id} required={field.required} help={field.help}>
      {(() => {
        switch (field.type) {
          case 'code':
            return (
              <CodeField
                id={id}
                value={str(value)}
                onChange={onChange}
                language={field.language}
                placeholder={field.placeholder}
              />
            )
          case 'textarea':
            return (
              <Textarea
                id={id}
                value={str(value)}
                placeholder={field.placeholder}
                onChange={(e) => onChange(e.target.value)}
              />
            )
          case 'number':
            return (
              <Input
                id={id}
                type="number"
                value={str(value)}
                placeholder={field.placeholder}
                onChange={(e) =>
                  onChange(e.target.value === '' ? undefined : e.target.valueAsNumber)
                }
              />
            )
          case 'select':
            return (
              <Select id={id} value={str(value)} onChange={(e) => onChange(e.target.value)}>
                {field.options?.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </Select>
            )
          case 'keyvalue':
            return (
              <KeyValueField
                id={id}
                value={(value as Record<string, string> | undefined) ?? {}}
                onChange={onChange}
              />
            )
          case 'workflow':
            return <WorkflowField id={id} value={str(value)} onChange={onChange} />
          case 'inputs':
            return (
              <CallInputsField
                id={id}
                workflowId={config?.workflow as string | undefined}
                value={(value as Record<string, string> | undefined) ?? {}}
                onChange={onChange}
              />
            )
          default:
            return (
              <CodeField
                id={id}
                value={str(value)}
                onChange={onChange}
                placeholder={field.placeholder}
                singleLine
              />
            )
        }
      })()}
    </FormField>
  )
}
