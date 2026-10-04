import * as Dialog from '@radix-ui/react-dialog'
import { Play } from 'lucide-react'
import { useState, type FormEvent } from 'react'

import { Button } from '@/components/ui/Button'
import { FormField, Input } from '@/components/ui/Input'
import type { WorkflowInput } from '@/features/workflows/api'
import { cn } from '@/lib/cn'

type Values = Record<string, string | boolean>

type StartRunDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  workflowName: string
  inputs: WorkflowInput[]
  /** Values to start from, e.g. those of a previous run. */
  initial?: Record<string, unknown>
  pending: boolean
  errors: string[]
  onSubmit: (values: Record<string, unknown>) => void
}

function initialValues(inputs: WorkflowInput[], initial?: Record<string, unknown>): Values {
  return Object.fromEntries(
    inputs.map((i) => {
      const v = initial?.[i.name] ?? i.default
      if (i.type === 'boolean') return [i.name, v === true || v === 'true']
      return [i.name, v === undefined || v === null ? '' : String(v)]
    }),
  )
}

export function StartRunDialog({
  open,
  onOpenChange,
  workflowName,
  inputs,
  initial,
  pending,
  errors,
  onSubmit,
}: StartRunDialogProps) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/50 backdrop-blur-[2px] data-[state=open]:animate-fade-in" />
        <Dialog.Content className="fixed top-1/2 left-1/2 z-50 max-h-[85vh] w-[calc(100vw-32px)] max-w-md -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-xl border border-line-strong bg-raised p-5 shadow-2xl shadow-black/50 data-[state=open]:animate-pop-in">
          <Dialog.Title className="text-sm font-medium text-fg">Run {workflowName}</Dialog.Title>
          <Dialog.Description className="mt-1 text-fg-muted">
            Steps read these as <code className="font-mono text-xs">inputs.&lt;name&gt;</code>.
          </Dialog.Description>
          {/* Remount per opening so values reset to the latest initial values. */}
          {open && (
            <RunForm
              inputs={inputs}
              initial={initial}
              pending={pending}
              errors={errors}
              onSubmit={onSubmit}
            />
          )}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

function RunForm({
  inputs,
  initial,
  pending,
  errors,
  onSubmit,
}: Pick<StartRunDialogProps, 'inputs' | 'initial' | 'pending' | 'errors' | 'onSubmit'>) {
  const [values, setValues] = useState<Values>(() => initialValues(inputs, initial))
  const set = (name: string, v: string | boolean) => setValues((prev) => ({ ...prev, [name]: v }))

  const submit = (e: FormEvent) => {
    e.preventDefault()
    // The server applies defaults and types; blanks mean "use the default".
    onSubmit(Object.fromEntries(Object.entries(values).filter(([, v]) => v !== '')))
  }

  return (
    <form onSubmit={submit} className="mt-5 flex flex-col gap-4">
      {inputs.map((i) => {
        const id = `run-input-${i.name}`
        if (i.type === 'boolean') {
          const on = values[i.name] === true
          return (
            <label key={i.name} htmlFor={id} className="flex items-center justify-between gap-3">
              <span>
                <span className="block text-xs font-medium text-fg-muted">{i.label || i.name}</span>
                {i.description && (
                  <span className="block text-xs text-fg-subtle">{i.description}</span>
                )}
              </span>
              <button
                id={id}
                type="button"
                role="switch"
                aria-checked={on}
                onClick={() => set(i.name, !on)}
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
            <Input
              id={id}
              type={i.type === 'number' ? 'number' : 'text'}
              value={String(values[i.name] ?? '')}
              placeholder={i.default !== undefined ? String(i.default) : undefined}
              className={i.type === 'text' ? 'font-mono text-xs' : undefined}
              onChange={(e) => set(i.name, e.target.value)}
            />
          </FormField>
        )
      })}

      {errors.length > 0 && (
        <ul className="rounded-lg border border-coral/30 bg-coral/5 px-3 py-2 text-xs text-coral">
          {errors.map((e) => (
            <li key={e}>{e}</li>
          ))}
        </ul>
      )}

      <div className="mt-1 flex justify-end gap-2">
        <Dialog.Close asChild>
          <Button type="button" variant="ghost">
            Cancel
          </Button>
        </Dialog.Close>
        <Button type="submit" disabled={pending}>
          <Play className="size-3" fill="currentColor" />
          Run
        </Button>
      </div>
    </form>
  )
}
