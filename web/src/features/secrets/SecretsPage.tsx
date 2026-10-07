import { KeyRound, MoreHorizontal, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { toast } from 'sonner'

import { PageHeader } from '@/components/layout/PageHeader'
import { Button } from '@/components/ui/Button'
import { ConfirmDialog } from '@/components/ui/ConfirmDialog'
import { FormField, Input } from '@/components/ui/Input'
import { Menu, MenuContent, MenuItem, MenuTrigger } from '@/components/ui/Menu'
import { timeAgo } from '@/lib/time'

import { SECRET_NAME, useDeleteSecret, useSecrets, useSetSecret, type Secret } from './api'

export function SecretsPage() {
  const secrets = useSecrets()
  // null: closed; '': adding a new secret; a name: replacing that secret's value.
  const [editing, setEditing] = useState<string | null>(null)
  const [toDelete, setToDelete] = useState<Secret | null>(null)
  const del = useDeleteSecret()

  return (
    <>
      <PageHeader
        title="Secrets"
        actions={
          <Button onClick={() => setEditing('')}>
            <Plus className="size-3.5" strokeWidth={2.25} />
            New secret
          </Button>
        }
      />
      <div className="flex-1 overflow-y-auto">
        <div className="mx-auto flex w-full max-w-3xl flex-col gap-5 px-4 py-6 md:px-6">
          <p className="max-w-xl text-fg-muted">
            Use a secret in any step as{' '}
            <code className="rounded bg-raised px-1 py-0.5 font-mono text-xs text-brass">
              {'{{ secrets.NAME }}'}
            </code>
            . Values are encrypted with a key kept in your macOS Keychain, hidden in run logs, and
            can't be viewed again after saving — only replaced.
          </p>

          {editing !== null && (
            <SecretForm
              name={editing}
              onDone={() => setEditing(null)}
              taken={(secrets.data ?? []).map((s) => s.name)}
            />
          )}

          {secrets.isError ? (
            <p className="text-coral">Couldn't load secrets: {secrets.error.message}</p>
          ) : secrets.data && secrets.data.length === 0 && editing === null ? (
            <div className="flex flex-col items-center gap-3 rounded-xl border border-dashed border-line-strong px-6 py-10 text-center">
              <KeyRound className="size-5 text-fg-subtle" strokeWidth={1.75} />
              <div>
                <p className="font-medium text-fg">No secrets yet</p>
                <p className="mt-1 text-fg-muted">
                  Add API tokens and passwords here instead of pasting them into steps.
                </p>
              </div>
            </div>
          ) : (
            secrets.data &&
            secrets.data.length > 0 && (
              <ul className="divide-y divide-line overflow-hidden rounded-xl border border-line bg-surface">
                {secrets.data.map((s) => (
                  <li key={s.name} className="flex items-center gap-3 px-4 py-3">
                    <KeyRound className="size-3.5 shrink-0 text-fg-subtle" strokeWidth={1.75} />
                    <span className="min-w-0 flex-1 truncate font-mono text-xs text-fg">
                      {s.name}
                    </span>
                    <span className="font-mono text-xs tracking-widest text-fg-subtle select-none">
                      ••••••
                    </span>
                    <span
                      className="w-32 text-right text-xs text-fg-subtle"
                      title={new Date(s.updatedAt).toLocaleString()}
                    >
                      Updated {timeAgo(s.updatedAt)}
                    </span>
                    <Menu>
                      <MenuTrigger
                        aria-label={`Actions for ${s.name}`}
                        className="flex size-7 items-center justify-center rounded-md text-fg-subtle hover:bg-hover hover:text-fg data-[state=open]:bg-hover"
                      >
                        <MoreHorizontal className="size-4" />
                      </MenuTrigger>
                      <MenuContent>
                        <MenuItem
                          icon={<RefreshCw className="size-3.5" />}
                          onSelect={() => setEditing(s.name)}
                        >
                          Replace value
                        </MenuItem>
                        <MenuItem
                          destructive
                          icon={<Trash2 className="size-3.5" />}
                          onSelect={() => setToDelete(s)}
                        >
                          Delete
                        </MenuItem>
                      </MenuContent>
                    </Menu>
                  </li>
                ))}
              </ul>
            )
          )}
        </div>
      </div>

      <ConfirmDialog
        open={toDelete !== null}
        onOpenChange={(open) => !open && setToDelete(null)}
        title={`Delete ${toDelete?.name}?`}
        description="Workflows that use this secret won't be able to run until you add it again."
        confirmLabel="Delete"
        destructive
        pending={del.isPending}
        onConfirm={() =>
          toDelete &&
          del.mutate(toDelete.name, {
            onSuccess: () => {
              toast(`Deleted ${toDelete.name}`)
              setToDelete(null)
            },
            onError: (err) => toast.error(`Couldn't delete: ${err.message}`),
          })
        }
      />
    </>
  )
}

function SecretForm({
  name,
  taken,
  onDone,
}: {
  name: string
  taken: string[]
  onDone: () => void
}) {
  const replacing = name !== ''
  const [draftName, setDraftName] = useState(name)
  const [value, setValue] = useState('')
  const save = useSetSecret()

  const nameError =
    draftName === ''
      ? undefined
      : !SECRET_NAME.test(draftName)
        ? 'Use UPPER_SNAKE_CASE, starting with a letter.'
        : !replacing && taken.includes(draftName)
          ? 'A secret with this name exists. Use “Replace value” to change it.'
          : undefined

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (nameError || !draftName || !value) return
    save.mutate(
      { name: draftName, value },
      {
        onSuccess: () => {
          toast.success(replacing ? `Replaced ${draftName}` : `Saved ${draftName}`)
          onDone()
        },
        onError: (err) => toast.error(`Couldn't save: ${err.message}`),
      },
    )
  }

  return (
    <form
      onSubmit={submit}
      className="flex animate-pop-in flex-col gap-4 rounded-xl border border-line-strong bg-raised p-4"
    >
      <p className="text-sm font-medium text-fg">
        {replacing ? `Replace the value of ${name}` : 'New secret'}
      </p>
      <div className="grid gap-4 sm:grid-cols-2">
        <FormField label="Name" htmlFor="secret-name" error={nameError}>
          <Input
            id="secret-name"
            value={draftName}
            disabled={replacing}
            autoFocus={!replacing}
            placeholder="API_TOKEN"
            spellCheck={false}
            aria-invalid={!!nameError}
            className="font-mono text-xs"
            onChange={(e) => setDraftName(e.target.value.toUpperCase().replace(/[^A-Z0-9_]/g, '_'))}
          />
        </FormField>
        <FormField label="Value" htmlFor="secret-value">
          <Input
            id="secret-value"
            type="password"
            autoComplete="off"
            autoFocus={replacing}
            value={value}
            className="font-mono text-xs"
            onChange={(e) => setValue(e.target.value)}
          />
        </FormField>
      </div>
      <div className="flex justify-end gap-2">
        <Button type="button" variant="ghost" onClick={onDone}>
          Cancel
        </Button>
        <Button type="submit" disabled={!draftName || !value || !!nameError || save.isPending}>
          {replacing ? 'Replace value' : 'Save secret'}
        </Button>
      </div>
    </form>
  )
}
