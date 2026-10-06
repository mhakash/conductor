import * as Dialog from '@radix-ui/react-dialog'
import { FileUp } from 'lucide-react'
import { lazy, Suspense, useRef, useState, type ChangeEvent } from 'react'
import { useNavigate } from 'react-router'
import { toast } from 'sonner'

import { Button } from '@/components/ui/Button'
import { ApiError } from '@/lib/api'

import { useImportWorkflow } from './api'

// CodeMirror is heavy; the workflows page only needs it once this opens.
const CodeField = lazy(async () => ({
  default: (await import('@/features/editor/fields/CodeField')).CodeField,
}))

const example = `conductor: 1
name: Release
inputs:
  - name: version
    type: text
steps:
  - id: tag
    block: shell
    config:
      command: git tag v$VERSION
      env: {VERSION: "{{ inputs.version }}"}`

/** Creates a workflow from YAML, pasted or read from a file. */
export function ImportDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const [text, setText] = useState('')
  const [errors, setErrors] = useState<string[]>([])
  const file = useRef<HTMLInputElement>(null)
  const importWorkflow = useImportWorkflow()
  const navigate = useNavigate()

  const pick = async (e: ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0]
    e.target.value = ''
    if (!f) return
    setText(await f.text())
    setErrors([])
  }

  const submit = () =>
    importWorkflow.mutate(text, {
      onSuccess: (wf) => {
        toast.success(`Imported “${wf.name}”`)
        onOpenChange(false)
        navigate(`/workflows/${wf.id}`)
      },
      onError: (err) => {
        const problems = err instanceof ApiError ? err.problems.map((p) => p.message) : []
        setErrors(problems.length ? problems : [err.message])
      },
    })

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(o) => {
        onOpenChange(o)
        if (!o) setErrors([])
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/60 backdrop-blur-[2px] data-[state=open]:animate-fade-in" />
        <Dialog.Content className="fixed top-1/2 left-1/2 z-50 flex max-h-[calc(100vh-32px)] w-[calc(100vw-32px)] max-w-2xl -translate-x-1/2 -translate-y-1/2 flex-col rounded-xl border border-line-strong bg-raised p-5 shadow-2xl shadow-black/50 data-[state=open]:animate-pop-in">
          <Dialog.Title className="text-sm font-medium text-fg">Import a workflow</Dialog.Title>
          <Dialog.Description className="mt-1 text-fg-muted">
            Paste YAML exported from Conductor, or choose a file. It's added as a new workflow;
            steps that run other workflows find them by name.
          </Dialog.Description>
          <div className="mt-4 h-80 min-h-0">
            <Suspense fallback={null}>
              <CodeField
                id="import-yaml"
                language="yaml"
                literal
                fill
                value={text}
                placeholder={example}
                onChange={(v) => {
                  setText(v)
                  setErrors([])
                }}
              />
            </Suspense>
          </div>
          {errors.length > 0 && (
            <ul className="mt-3 flex max-h-28 flex-col gap-1 overflow-y-auto text-xs text-coral">
              {errors.map((e) => (
                <li key={e}>{e}</li>
              ))}
            </ul>
          )}
          <div className="mt-4 flex items-center gap-2">
            <input
              ref={file}
              type="file"
              accept=".yaml,.yml,application/yaml,text/yaml"
              className="hidden"
              onChange={pick}
            />
            <Button variant="ghost" onClick={() => file.current?.click()}>
              <FileUp className="size-3.5" /> Choose file
            </Button>
            <div className="ml-auto flex gap-2">
              <Dialog.Close asChild>
                <Button variant="ghost">Cancel</Button>
              </Dialog.Close>
              <Button onClick={submit} disabled={!text.trim() || importWorkflow.isPending}>
                Import
              </Button>
            </div>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
