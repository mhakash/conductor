import * as Dialog from '@radix-ui/react-dialog'
import { Maximize2, X } from 'lucide-react'
import { useState } from 'react'

import { CodeField } from './CodeField'

type ExpandableCodeFieldProps = {
  id: string
  label: string
  value: string
  onChange: (value: string) => void
  language?: string
  placeholder?: string
  literal?: boolean
}

/** A code field that can be opened in a large editor, for scripts longer than a few lines. */
export function ExpandableCodeField({ id, label, ...field }: ExpandableCodeFieldProps) {
  const [open, setOpen] = useState(false)

  return (
    <div className="group relative">
      <CodeField id={id} {...field} />
      <button
        type="button"
        aria-label={`Open ${label.toLowerCase()} in a larger editor`}
        title="Open in a larger editor"
        onClick={() => setOpen(true)}
        className="absolute top-1.5 right-1.5 flex size-6 items-center justify-center rounded-md bg-raised/80 text-fg-subtle opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100 hover:text-fg focus-visible:opacity-100"
      >
        <Maximize2 className="size-3" />
      </button>

      <Dialog.Root open={open} onOpenChange={setOpen}>
        <Dialog.Portal>
          <Dialog.Overlay className="fixed inset-0 z-40 bg-black/60 backdrop-blur-[2px] data-[state=open]:animate-fade-in" />
          <Dialog.Content
            aria-describedby={undefined}
            className="fixed inset-4 z-50 mx-auto flex max-w-5xl flex-col overflow-hidden rounded-xl border border-line-strong bg-raised shadow-2xl shadow-black/50 data-[state=open]:animate-pop-in md:inset-10"
          >
            <div className="flex h-11 shrink-0 items-center gap-3 border-b border-line px-4">
              <Dialog.Title className="text-sm font-medium text-fg">{label}</Dialog.Title>
              <span className="text-xs text-fg-subtle">Changes apply as you type.</span>
              <Dialog.Close
                aria-label="Close"
                className="ml-auto flex size-7 items-center justify-center rounded-md text-fg-subtle hover:bg-hover hover:text-fg"
              >
                <X className="size-4" />
              </Dialog.Close>
            </div>
            <div className="min-h-0 flex-1 p-3">
              <CodeField id={`${id}-expanded`} {...field} fill autoFocus />
            </div>
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
    </div>
  )
}
