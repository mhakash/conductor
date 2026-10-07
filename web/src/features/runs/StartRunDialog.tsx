import * as Dialog from '@radix-ui/react-dialog'
import { Play } from 'lucide-react'
import { useState, type FormEvent } from 'react'

import { Button } from '@/components/ui/Button'
import type { WorkflowInput } from '@/features/workflows/api'

import { InputFields } from './InputFields'
import { filledValues, initialValues, type InputValues } from './inputValues'

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
  const [values, setValues] = useState<InputValues>(() => initialValues(inputs, initial))

  const submit = (e: FormEvent) => {
    e.preventDefault()
    onSubmit(filledValues(values))
  }

  return (
    <form onSubmit={submit} className="mt-5 flex flex-col gap-4">
      <InputFields
        inputs={inputs}
        values={values}
        onChange={(name, v) => setValues((prev) => ({ ...prev, [name]: v }))}
        idPrefix="run-input"
      />

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
