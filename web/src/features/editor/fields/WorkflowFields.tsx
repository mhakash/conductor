import { ArrowUpRight, X } from 'lucide-react'
import { Link } from 'react-router'

import { Select } from '@/components/ui/Select'
import { useWorkflowMap } from '@/features/workflows/api'

import { useEditor } from '../store'
import { CodeField } from './CodeField'

/** Picks the workflow a step runs, from every workflow but this one. */
export function WorkflowField({
  id,
  value,
  onChange,
}: {
  id: string
  value: string
  onChange: (id: string) => void
}) {
  const current = useEditor((s) => s.workflowId)
  const workflows = useWorkflowMap()
  const deleted = value !== '' && workflows.size > 0 && !workflows.has(value)

  return (
    <div className="flex gap-1.5">
      <Select
        id={id}
        value={value}
        aria-invalid={deleted}
        onValueChange={onChange}
        placeholder="Choose a workflow"
        options={[
          ...(deleted ? [{ value, label: 'A deleted workflow' }] : []),
          ...[...workflows.values()]
            .filter((w) => w.id !== current)
            .map((w) => ({ value: w.id, label: w.name })),
        ]}
      />
      {value && !deleted && (
        <Link
          to={`/workflows/${value}`}
          aria-label="Open this workflow"
          title="Open this workflow"
          className="flex size-8 shrink-0 items-center justify-center rounded-md text-fg-subtle hover:bg-hover hover:text-fg"
        >
          <ArrowUpRight className="size-3.5" />
        </Link>
      )}
    </div>
  )
}

/**
 * Values for the inputs of the workflow a step runs, one field per input it
 * declares. An empty field means the input's default.
 */
export function CallInputsField({
  id,
  workflowId,
  value,
  onChange,
}: {
  id: string
  workflowId: string | undefined
  value: Record<string, string>
  onChange: (value: Record<string, string>) => void
}) {
  const target = useWorkflowMap().get(workflowId ?? '')
  if (!target) {
    return <p className="text-xs text-fg-subtle">Choose a workflow to see its inputs.</p>
  }

  const set = (name: string, v: string) => {
    const next = { ...value }
    if (v === '') delete next[name]
    else next[name] = v
    onChange(next)
  }
  const declared = new Set(target.inputs.map((i) => i.name))
  const stale = Object.keys(value).filter((name) => !declared.has(name))

  return (
    <div className="flex flex-col gap-3">
      {target.inputs.length === 0 && (
        <p className="text-xs text-fg-subtle">{target.name} has no inputs.</p>
      )}
      {target.inputs.map((input) => (
        <div key={input.name} className="flex flex-col gap-1">
          <label htmlFor={`${id}-${input.name}`} className="flex items-baseline gap-1.5 text-xs">
            <span className="font-mono text-fg">{input.name}</span>
            {input.required && input.default === undefined && <span className="text-brass">*</span>}
            {input.label && <span className="truncate text-fg-subtle">{input.label}</span>}
          </label>
          <CodeField
            id={`${id}-${input.name}`}
            singleLine
            value={value[input.name] ?? ''}
            placeholder={
              input.default !== undefined
                ? `Default: ${String(input.default)}`
                : input.type === 'text'
                  ? undefined
                  : input.type === 'number'
                    ? 'A number'
                    : 'true or false'
            }
            onChange={(v) => set(input.name, v)}
          />
        </div>
      ))}
      {stale.map((name) => (
        <p key={name} className="flex items-center gap-1.5 text-xs text-coral">
          <span className="min-w-0 flex-1">
            Gives <code className="font-mono">{name}</code>, which {target.name} no longer has.
          </span>
          <button
            aria-label={`Remove ${name}`}
            onClick={() => set(name, '')}
            className="flex size-6 items-center justify-center rounded-md hover:bg-hover"
          >
            <X className="size-3" />
          </button>
        </p>
      ))}
    </div>
  )
}
