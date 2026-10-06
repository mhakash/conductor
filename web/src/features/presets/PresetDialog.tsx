import * as Dialog from '@radix-ui/react-dialog'
import { useState, type FormEvent } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/Button'
import { FormField, Input, Textarea } from '@/components/ui/Input'

import { usePresets, useSavePreset, type Preset, type PresetFields } from './api'

type PresetDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
} & (
  | /** Renames or redescribes an existing preset. */ { preset: Preset; draft?: never }
  | /** Saves a step as a preset; a preset with the same name is replaced. */ {
      draft: PresetFields
      preset?: never
    }
)

export function PresetDialog({ open, onOpenChange, preset, draft }: PresetDialogProps) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/50 backdrop-blur-[2px] data-[state=open]:animate-fade-in" />
        <Dialog.Content className="fixed top-1/2 left-1/2 z-50 w-[calc(100vw-32px)] max-w-sm -translate-x-1/2 -translate-y-1/2 rounded-xl border border-line-strong bg-raised p-5 shadow-2xl shadow-black/50 data-[state=open]:animate-pop-in">
          <Dialog.Title className="text-sm font-medium text-fg">
            {preset ? 'Edit preset' : 'Save as preset'}
          </Dialog.Title>
          <Dialog.Description className="mt-1 text-fg-muted">
            {preset
              ? 'Steps already made from this preset keep their own copy.'
              : 'Its configuration, timeout and retries are saved. Add it from the library in any workflow.'}
          </Dialog.Description>
          {/* Remount per opening so the fields start from the current values. */}
          {open && <PresetForm preset={preset} draft={draft} onDone={() => onOpenChange(false)} />}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

function PresetForm({
  preset,
  draft,
  onDone,
}: {
  preset?: Preset
  draft?: PresetFields
  onDone: () => void
}) {
  const presets = usePresets().data ?? []
  const save = useSavePreset()
  const base = preset ?? draft!
  const [name, setName] = useState(base.name)
  const [description, setDescription] = useState(base.description)

  const clash = presets.find(
    (p) => p.id !== preset?.id && p.name.toLowerCase() === name.trim().toLowerCase(),
  )
  // Saving a step under a taken name replaces that preset; renaming onto one isn't allowed.
  const replaces = !preset && clash
  const blocked = !name.trim() || (preset && clash)

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (blocked) return
    save.mutate(
      {
        id: preset?.id ?? clash?.id,
        name: name.trim(),
        description: description.trim(),
        block: base.block,
        step: base.step,
      },
      {
        onSuccess: (p) => {
          toast.success(
            preset ? 'Preset saved' : replaces ? `Replaced “${p.name}”` : `Saved “${p.name}”`,
          )
          onDone()
        },
        onError: (err) => toast.error(`Couldn't save the preset: ${err.message}`),
      },
    )
  }

  return (
    <form onSubmit={submit} className="mt-4 flex flex-col gap-4">
      <FormField
        label="Name"
        htmlFor="preset-name"
        required
        error={preset && clash ? 'Another preset already has this name.' : undefined}
        help={replaces ? `Replaces the preset called “${clash.name}”.` : undefined}
      >
        <Input
          id="preset-name"
          autoFocus
          value={name}
          aria-invalid={!!(preset && clash)}
          onChange={(e) => setName(e.target.value)}
        />
      </FormField>
      <FormField label="Description" htmlFor="preset-description">
        <Textarea
          id="preset-description"
          rows={2}
          className="min-h-0"
          placeholder="Shown in the library"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
      </FormField>
      <div className="flex justify-end gap-2">
        <Dialog.Close asChild>
          <Button type="button" variant="ghost">
            Cancel
          </Button>
        </Dialog.Close>
        <Button type="submit" disabled={!!blocked || save.isPending}>
          {replaces ? 'Replace preset' : 'Save preset'}
        </Button>
      </div>
    </form>
  )
}
