import { Bookmark, MoreHorizontal, Pencil, Trash2 } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { toast } from 'sonner'

import { PageHeader } from '@/components/layout/PageHeader'
import { ConfirmDialog } from '@/components/ui/ConfirmDialog'
import { Menu, MenuContent, MenuItem, MenuTrigger } from '@/components/ui/Menu'
import { BlockIcon } from '@/features/blocks/BlockIcon'
import { CALL_BLOCK, useBlockMap, useBlocks, type Block } from '@/features/blocks/api'
import { useWorkflowMap } from '@/features/workflows/api'

import { useDeletePreset, usePresets, type Preset } from '@/features/presets/api'
import { PresetDialog } from '@/features/presets/PresetDialog'

/**
 * Everything workflows are built from, outside any one workflow: your
 * presets, and a reference for each block. Other workflows show up in the
 * editor's library too, but are managed on the Workflows page.
 */
export function LibraryPage() {
  return (
    <>
      <PageHeader title="Library" />
      <div className="flex-1 overflow-y-auto">
        <div className="mx-auto flex w-full max-w-3xl flex-col gap-10 px-4 py-6 md:px-6">
          <Presets />
          <Blocks />
        </div>
      </div>
    </>
  )
}

function SectionTitle({ id, title, children }: { id: string; title: string; children: ReactNode }) {
  return (
    <div className="mb-3">
      <h2 id={id} className="text-base font-medium text-fg">
        {title}
      </h2>
      <p className="mt-1 max-w-xl text-fg-muted">{children}</p>
    </div>
  )
}

function Presets() {
  const presets = usePresets()
  const blocks = useBlockMap()
  const [editing, setEditing] = useState<Preset | null>(null)
  const [toDelete, setToDelete] = useState<Preset | null>(null)
  const del = useDeletePreset()

  const confirmDelete = () => {
    if (!toDelete) return
    del.mutate(toDelete.id, {
      onSuccess: () => {
        toast(`Deleted “${toDelete.name}”`)
        setToDelete(null)
      },
      onError: (err) => toast.error(`Couldn't delete the preset: ${err.message}`),
    })
  }

  return (
    <section aria-labelledby="presets">
      <SectionTitle id="presets" title="Presets">
        Steps saved with their configuration, ready to add from the library in the editor. To make
        one, select a step and choose Save as preset.
      </SectionTitle>
      {presets.isError ? (
        <p className="text-coral">Couldn't load presets: {presets.error.message}</p>
      ) : presets.data?.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-xl border border-dashed border-line-strong px-6 py-8 text-center">
          <Bookmark className="size-5 text-fg-subtle" strokeWidth={1.75} />
          <p className="text-fg-muted">No presets yet.</p>
        </div>
      ) : (
        <ul className="divide-y divide-line overflow-hidden rounded-xl border border-line bg-surface">
          {presets.data?.map((p) => (
            <PresetRow
              key={p.id}
              preset={p}
              block={blocks.get(p.block)}
              onEdit={() => setEditing(p)}
              onDelete={() => setToDelete(p)}
            />
          ))}
        </ul>
      )}

      {editing && (
        <PresetDialog open onOpenChange={(open) => !open && setEditing(null)} preset={editing} />
      )}
      <ConfirmDialog
        open={toDelete !== null}
        onOpenChange={(open) => !open && setToDelete(null)}
        title="Delete preset?"
        description={
          <>“{toDelete?.name}” will be removed from the library. Steps made from it are kept.</>
        }
        confirmLabel="Delete"
        destructive
        pending={del.isPending}
        onConfirm={confirmDelete}
      />
    </section>
  )
}

function PresetRow({
  preset,
  block,
  onEdit,
  onDelete,
}: {
  preset: Preset
  block: Block | undefined
  onEdit: () => void
  onDelete: () => void
}) {
  const workflows = useWorkflowMap()
  const { config, timeout, retry, continueOnError } = preset.step
  const settings = [
    timeout && `timeout ${timeout}`,
    retry && `${retry.max} ${retry.max === 1 ? 'retry' : 'retries'}`,
    continueOnError && 'may fail',
  ].filter(Boolean)
  const shown = (key: string, value: unknown) =>
    preset.block === CALL_BLOCK && key === 'workflow'
      ? (workflows.get(value as string)?.name ?? 'a deleted workflow')
      : typeof value === 'string'
        ? value
        : JSON.stringify(value)
  const entries = Object.entries(config).filter(
    ([, v]) => v !== '' && v != null && !(typeof v === 'object' && Object.keys(v).length === 0),
  )

  return (
    <li className="flex items-start gap-3 px-4 py-3">
      <span className="mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-md bg-hover text-fg-muted">
        <BlockIcon name={block?.icon ?? ''} className="size-3.5" />
      </span>
      <div className="min-w-0 flex-1">
        <p className="flex flex-wrap items-baseline gap-x-2">
          <span className="font-medium text-fg">{preset.name}</span>
          <span className="font-mono text-[11px] text-fg-subtle">
            {block?.name ?? preset.block}
            {settings.length > 0 && ` · ${settings.join(' · ')}`}
          </span>
        </p>
        {preset.description && <p className="text-fg-muted">{preset.description}</p>}
        {entries.length > 0 && (
          <dl className="mt-1.5 grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 font-mono text-xs">
            {entries.map(([k, v]) => (
              <div key={k} className="contents">
                <dt className="text-fg-subtle">{k}</dt>
                <dd className="truncate text-fg-muted">{shown(k, v)}</dd>
              </div>
            ))}
          </dl>
        )}
      </div>
      <Menu>
        <MenuTrigger
          aria-label={`Actions for ${preset.name}`}
          className="flex size-7 shrink-0 items-center justify-center rounded-md text-fg-subtle hover:bg-hover hover:text-fg data-[state=open]:bg-hover"
        >
          <MoreHorizontal className="size-4" />
        </MenuTrigger>
        <MenuContent>
          <MenuItem icon={<Pencil className="size-3.5" />} onSelect={onEdit}>
            Rename
          </MenuItem>
          <MenuItem destructive icon={<Trash2 className="size-3.5" />} onSelect={onDelete}>
            Delete
          </MenuItem>
        </MenuContent>
      </Menu>
    </li>
  )
}

const fieldTypes: Record<string, string> = {
  text: 'text',
  textarea: 'text',
  code: 'code',
  number: 'number',
  boolean: 'yes / no',
  select: 'choice',
  keyvalue: 'key–value pairs',
  workflow: 'workflow',
  inputs: 'inputs',
}

function Blocks() {
  const blocks = useBlocks().data ?? []
  return (
    <section aria-labelledby="blocks">
      <SectionTitle id="blocks" title="Blocks">
        What each kind of step needs and produces. Later steps read outputs as{' '}
        <code className="font-mono text-xs text-brass">steps.id.outputs.name</code>.
      </SectionTitle>
      <ul className="flex flex-col gap-3">
        {blocks.map((b) => (
          <li key={b.type} className="rounded-xl border border-line bg-surface p-4">
            <div className="flex items-start gap-3">
              <span className="flex size-7 shrink-0 items-center justify-center rounded-md bg-hover text-brass">
                <BlockIcon name={b.icon} className="size-3.5" />
              </span>
              <div className="min-w-0">
                <p className="flex items-baseline gap-2">
                  <span className="font-medium text-fg">{b.name}</span>
                  <span className="font-mono text-[11px] text-fg-subtle">{b.type}</span>
                </p>
                <p className="text-fg-muted">{b.description}</p>
              </div>
            </div>
            <div className="mt-3 grid gap-4 sm:grid-cols-2">
              <Reference
                title="Configuration"
                rows={b.fields.map((f) => ({
                  key: f.label + (f.required ? ' *' : ''),
                  value: fieldTypes[f.type] ?? f.type,
                }))}
                empty="Nothing to configure"
              />
              <Reference
                title="Outputs"
                rows={b.outputs.map((o) => ({ key: o.key, value: o.description, mono: true }))}
                empty={b.type === CALL_BLOCK ? 'The outputs of the workflow it runs' : 'No outputs'}
              />
            </div>
          </li>
        ))}
      </ul>
    </section>
  )
}

function Reference({
  title,
  rows,
  empty,
}: {
  title: string
  rows: { key: string; value: string; mono?: boolean }[]
  empty: string
}) {
  return (
    <div>
      <h3 className="mb-1.5 text-[11px] font-medium tracking-wide text-fg-subtle uppercase">
        {title}
      </h3>
      {rows.length === 0 ? (
        <p className="text-xs text-fg-subtle">{empty}</p>
      ) : (
        <dl className="flex flex-col gap-1 text-xs">
          {rows.map((r) => (
            <div key={r.key} className="flex gap-2">
              <dt className={r.mono ? 'shrink-0 font-mono text-fg' : 'shrink-0 text-fg'}>
                {r.key}
              </dt>
              <dd className="text-fg-subtle">{r.value}</dd>
            </div>
          ))}
        </dl>
      )}
    </div>
  )
}
