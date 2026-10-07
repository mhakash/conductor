import { Plus, X } from 'lucide-react'

import { Button } from '@/components/ui/Button'
import { Input, Select, Textarea } from '@/components/ui/Input'
import type { InputType, WorkflowInput } from '@/features/workflows/api'

const inputTypes: { value: InputType; label: string }[] = [
  { value: 'text', label: 'Text' },
  { value: 'longtext', label: 'Long text' },
  { value: 'number', label: 'Number' },
  { value: 'boolean', label: 'Yes / no' },
  { value: 'select', label: 'Choice' },
]

/**
 * Edits a list of input definitions: the workflow's inputs, or the fields of
 * an input step. `noun` names one in labels; with `exprDefaults`, defaults
 * are typed as text so they can hold {{ }}.
 */
export function InputDefsEditor({
  value,
  onChange,
  noun,
  firstName,
  exprDefaults = false,
}: {
  value: WorkflowInput[]
  onChange: (value: WorkflowInput[]) => void
  noun: string
  firstName: string
  exprDefaults?: boolean
}) {
  const update = (i: number, patch: Partial<WorkflowInput>) =>
    onChange(value.map((input, j) => (j === i ? { ...input, ...patch } : input)))

  const add = () => {
    const taken = new Set(value.map((i) => i.name))
    let name = firstName
    for (let n = 2; taken.has(name); n++) name = `${noun}${n}`
    onChange([...value, { name, type: 'text', required: false }])
  }

  return (
    <>
      {value.map((input, i) => (
        <div
          key={i}
          className="flex flex-col gap-2 rounded-lg border border-line bg-canvas/50 p-2.5"
        >
          <div className="flex gap-1.5">
            <Input
              aria-label={`${noun} name`}
              value={input.name}
              spellCheck={false}
              className="font-mono text-xs"
              onChange={(e) => update(i, { name: e.target.value })}
            />
            <Select
              aria-label="Type"
              value={input.type}
              className="w-28 shrink-0"
              onChange={(e) => {
                const type = e.target.value as InputType
                update(i, {
                  type,
                  default: undefined,
                  options: type === 'select' ? (input.options ?? []) : undefined,
                })
              }}
            >
              {inputTypes.map((t) => (
                <option key={t.value} value={t.value}>
                  {t.label}
                </option>
              ))}
            </Select>
            <button
              aria-label={`Remove ${noun} ${input.name}`}
              onClick={() => onChange(value.filter((_, j) => j !== i))}
              className="flex size-8 shrink-0 items-center justify-center rounded-md text-fg-subtle hover:bg-hover hover:text-fg"
            >
              <X className="size-3.5" />
            </button>
          </div>
          <Input
            aria-label="Label"
            placeholder="Label (optional)"
            value={input.label ?? ''}
            onChange={(e) => update(i, { label: e.target.value || undefined })}
          />
          {input.type === 'select' && (
            <Textarea
              aria-label="Options"
              placeholder="Options, one per line"
              rows={3}
              value={(input.options ?? []).join('\n')}
              onChange={(e) => update(i, { options: e.target.value.split('\n') })}
              onBlur={() =>
                update(i, { options: (input.options ?? []).map((o) => o.trim()).filter(Boolean) })
              }
            />
          )}
          <div className="flex items-center gap-1.5">
            <DefaultValue
              input={input}
              exprDefaults={exprDefaults}
              onChange={(d) => update(i, { default: d })}
            />
            <label className="flex shrink-0 items-center gap-1.5 px-1 text-xs text-fg-muted">
              <input
                type="checkbox"
                checked={!!input.required}
                onChange={(e) => update(i, { required: e.target.checked })}
                className="accent-brass"
              />
              Required
            </label>
          </div>
        </div>
      ))}
      <Button variant="ghost" className="w-fit" onClick={add}>
        <Plus className="size-3.5" /> Add {noun}
      </Button>
    </>
  )
}

function DefaultValue({
  input,
  exprDefaults,
  onChange,
}: {
  input: WorkflowInput
  exprDefaults: boolean
  onChange: (value: WorkflowInput['default']) => void
}) {
  if (input.type === 'boolean') {
    return (
      <Select
        aria-label="Default"
        value={input.default === true ? 'true' : 'false'}
        onChange={(e) => onChange(e.target.value === 'true')}
      >
        <option value="false">Default: no</option>
        <option value="true">Default: yes</option>
      </Select>
    )
  }
  if (input.type === 'select') {
    return (
      <Select
        aria-label="Default"
        value={input.default === undefined ? '' : String(input.default)}
        onChange={(e) => onChange(e.target.value === '' ? undefined : e.target.value)}
      >
        <option value="">No default</option>
        {(input.options ?? []).filter(Boolean).map((o) => (
          <option key={o} value={o}>
            {o}
          </option>
        ))}
      </Select>
    )
  }
  const number = input.type === 'number' && !exprDefaults
  return (
    <Input
      aria-label="Default value"
      placeholder="Default (optional)"
      type={number ? 'number' : 'text'}
      value={input.default === undefined ? '' : String(input.default)}
      onChange={(e) =>
        onChange(
          e.target.value === '' ? undefined : number ? e.target.valueAsNumber : e.target.value,
        )
      }
    />
  )
}
