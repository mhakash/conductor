import { FormField, Input, Select, Textarea } from '@/components/ui/Input'
import type { Field } from '@/features/blocks/api'
import { cn } from '@/lib/cn'

import type { WorkflowInput } from '@/features/workflows/api'

import { CodeField } from './CodeField'
import { ExpandableCodeField } from './ExpandableCodeField'
import { InputDefsEditor } from './InputDefsEditor'
import { KeyValueField } from './KeyValueField'
import { CallInputsField, WorkflowField } from './WorkflowFields'

type ConfigFieldProps = {
  field: Field
  value: unknown
  onChange: (value: unknown) => void
  /** The step's whole config, for fields that depend on another (inputs on workflow). */
  config?: Record<string, unknown>
}

// Editor modes for the script block's languages; other values name a mode.
const editorLanguages: Record<string, string> = { node: 'javascript', bash: 'shell' }

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
          case 'code': {
            const picked = field.languageFrom ? str(config?.[field.languageFrom]) : ''
            const language = picked ? (editorLanguages[picked] ?? picked) : field.language
            return (
              <ExpandableCodeField
                id={id}
                label={field.label}
                value={str(value)}
                onChange={onChange}
                language={language}
                placeholder={field.placeholders?.[picked] ?? field.placeholder}
                literal={field.literal}
              />
            )
          }
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
          case 'form':
            return (
              <div id={id} className="flex flex-col gap-2">
                <InputDefsEditor
                  value={(value as WorkflowInput[] | undefined) ?? []}
                  onChange={onChange}
                  noun="field"
                  firstName="answer"
                  exprDefaults
                />
              </div>
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
