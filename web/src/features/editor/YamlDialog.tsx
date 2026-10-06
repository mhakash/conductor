import * as Dialog from '@radix-ui/react-dialog'
import { Check, Copy, Download, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/Button'
import { fromYAML, toYAML } from '@/features/workflows/api'
import { ApiError } from '@/lib/api'

import { CodeField } from './fields/CodeField'
import { toDefinition } from './graph'
import { useEditor } from './store'

/**
 * The draft as YAML: to read, copy or download, or to edit and apply back
 * to the canvas. Applying changes the draft only; saving is still separate.
 */
export function YamlDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/60 backdrop-blur-[2px] data-[state=open]:animate-fade-in" />
        <Dialog.Content className="fixed inset-4 z-50 mx-auto flex max-w-4xl flex-col overflow-hidden rounded-xl border border-line-strong bg-raised shadow-2xl shadow-black/50 data-[state=open]:animate-pop-in md:inset-10">
          {open && <YamlView onClose={() => onOpenChange(false)} />}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

function YamlView({ onClose }: { onClose: () => void }) {
  const [original, setOriginal] = useState<string | null>(null)
  const [text, setText] = useState('')
  const [errors, setErrors] = useState<string[]>([])
  const [busy, setBusy] = useState(false)
  const [copied, setCopied] = useState(false)
  const name = useEditor((s) => s.name)

  useEffect(() => {
    const s = useEditor.getState()
    toYAML({
      name: s.name,
      description: s.description,
      definition: toDefinition(s.nodes, s.edges, s.inputs, s.outputs),
    }).then(
      (yaml) => {
        setOriginal(yaml)
        setText(yaml)
      },
      (err: Error) => setErrors([err.message]),
    )
  }, [])

  const edited = original !== null && text !== original

  const apply = async () => {
    setBusy(true)
    try {
      useEditor.getState().replaceDraft(await fromYAML(text))
      toast('Applied to the workflow. Save to keep it.')
      onClose()
    } catch (err) {
      const problems = err instanceof ApiError ? err.problems.map((p) => p.message) : []
      setErrors(problems.length ? problems : [(err as Error).message])
    } finally {
      setBusy(false)
    }
  }

  const copy = async () => {
    await navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  const download = () => {
    const url = URL.createObjectURL(new Blob([text], { type: 'application/yaml' }))
    const a = document.createElement('a')
    a.href = url
    a.download = `${fileName(name)}.yaml`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <>
      <div className="flex h-11 shrink-0 items-center gap-3 border-b border-line px-4">
        <Dialog.Title className="text-sm font-medium text-fg">YAML</Dialog.Title>
        <Dialog.Description className="hidden truncate text-xs text-fg-subtle sm:block">
          {edited
            ? 'Edited. Apply to update the canvas.'
            : 'This workflow as it is now, unsaved changes included. Edit it here and apply.'}
        </Dialog.Description>
        <Dialog.Close
          aria-label="Close"
          className="ml-auto flex size-7 items-center justify-center rounded-md text-fg-subtle hover:bg-hover hover:text-fg"
        >
          <X className="size-4" />
        </Dialog.Close>
      </div>
      <div className="min-h-0 flex-1 p-3">
        {original !== null && (
          <CodeField
            id="workflow-yaml"
            language="yaml"
            literal
            fill
            value={text}
            onChange={(v) => {
              setText(v)
              setErrors([])
            }}
          />
        )}
      </div>
      {errors.length > 0 && (
        <ul className="flex max-h-32 shrink-0 flex-col gap-1 overflow-y-auto border-t border-line bg-coral/5 px-4 py-2.5 text-xs text-coral">
          {errors.map((e) => (
            <li key={e}>{e}</li>
          ))}
        </ul>
      )}
      <div className="flex shrink-0 flex-wrap items-center gap-1.5 border-t border-line px-3 py-2.5">
        <Button variant="ghost" onClick={copy} disabled={original === null}>
          {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
          {copied ? 'Copied' : 'Copy'}
        </Button>
        <Button variant="ghost" onClick={download} disabled={original === null}>
          <Download className="size-3.5" /> Download
        </Button>
        <div className="ml-auto flex gap-1.5">
          {edited && (
            <Button variant="ghost" onClick={() => setText(original)}>
              Discard edits
            </Button>
          )}
          <Button onClick={apply} disabled={!edited || busy}>
            Apply to workflow
          </Button>
        </div>
      </div>
    </>
  )
}

/** "Build prep" → "build-prep", as the server names exported files. */
function fileName(name: string) {
  return (
    name
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '') || 'workflow'
  )
}
