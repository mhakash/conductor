import { ChevronLeft, ChevronRight } from 'lucide-react'
import { motion } from 'motion/react'
import type { ReactNode } from 'react'
import { Link, useSearchParams } from 'react-router'

import { PageHeader } from '@/components/layout/PageHeader'
import { Button } from '@/components/ui/Button'
import { Select } from '@/components/ui/Input'
import { useWorkflows } from '@/features/workflows/api'
import { cn } from '@/lib/cn'
import { elapsed, formatDuration, timeAgo } from '@/lib/time'
import { useNow } from '@/lib/useNow'

import { isActive, statusLabel, useRunPage, type RunStatus, type RunSummary } from './api'
import { StatusIcon } from './StatusIcon'

const PAGE_SIZE = 25

const STATUSES: RunStatus[] = ['running', 'waiting', 'succeeded', 'failed', 'cancelled', 'queued']

export function RunsPage() {
  // Filters and page live in the URL so going back from a run returns here.
  const [params, setParams] = useSearchParams()
  // Scratch runs have no workflow, so the two filters exclude each other.
  const scratch = params.get('scratch') === 'true'
  const workflowId = scratch ? undefined : (params.get('workflow') ?? undefined)
  const status = (params.get('status') as RunStatus | null) ?? undefined
  const page = Math.max(Number(params.get('page')) || 1, 1) - 1
  const filtered = scratch || !!workflowId || !!status

  const runs = useRunPage({ workflowId, scratch, status, page, pageSize: PAGE_SIZE })

  // Any change other than the page itself starts again from the first page.
  const update = (changes: Record<string, string | undefined>) =>
    setParams(
      (p) => {
        const next = new URLSearchParams(p)
        for (const [key, value] of Object.entries(changes)) {
          if (value) next.set(key, value)
          else next.delete(key)
        }
        if (!('page' in changes)) next.delete('page')
        return next
      },
      { replace: !('page' in changes) },
    )

  return (
    <>
      <PageHeader title="Runs" />
      <div className="flex flex-1 flex-col overflow-y-auto">
        {(filtered || page > 0 || !!runs.data?.runs.length) && (
          <Filters
            workflowId={workflowId}
            scratch={scratch}
            status={status}
            onWorkflow={(v) => update({ workflow: v, scratch: undefined })}
            onScratch={(on) => update({ scratch: on ? 'true' : undefined, workflow: undefined })}
            onStatus={(v) => update({ status: v })}
          />
        )}
        {runs.isPending ? null : runs.isError ? (
          <Centered title="Couldn't load runs">{runs.error.message}</Centered>
        ) : runs.data.runs.length === 0 ? (
          filtered || page > 0 ? (
            <Centered title="No matching runs">
              Nothing here with these filters.{' '}
              <button
                type="button"
                className="text-brass hover:underline"
                onClick={() => setParams({})}
              >
                Clear filters
              </button>
            </Centered>
          ) : (
            <Centered title="No runs yet">
              Run a workflow from its editor. Each run shows up here with its logs, outputs and step
              timings.
            </Centered>
          )
        ) : (
          <div className="mx-auto w-full max-w-4xl px-4 pb-6 md:px-6">
            <ul
              className={cn(
                'divide-y divide-line overflow-hidden rounded-xl border border-line bg-surface transition-opacity',
                runs.isPlaceholderData && 'opacity-60',
              )}
            >
              {runs.data.runs.map((r, i) => (
                <RunRow key={r.id} run={r} index={i} />
              ))}
            </ul>
            {(page > 0 || runs.data.hasNext) && (
              <nav
                aria-label="Pages"
                className="mt-4 flex items-center justify-between gap-3 text-xs text-fg-subtle"
              >
                <span className="tabular-nums">
                  {page * PAGE_SIZE + 1}–{page * PAGE_SIZE + runs.data.runs.length}
                </span>
                <div className="flex items-center gap-2">
                  <Button
                    variant="secondary"
                    disabled={page === 0}
                    onClick={() => update({ page: page > 1 ? String(page) : undefined })}
                  >
                    <ChevronLeft className="size-3.5" /> Newer
                  </Button>
                  <span className="tabular-nums">Page {page + 1}</span>
                  <Button
                    variant="secondary"
                    disabled={!runs.data.hasNext || runs.isPlaceholderData}
                    onClick={() => update({ page: String(page + 2) })}
                  >
                    Older <ChevronRight className="size-3.5" />
                  </Button>
                </div>
              </nav>
            )}
          </div>
        )}
      </div>
    </>
  )
}

type FiltersProps = {
  workflowId?: string
  scratch: boolean
  status?: RunStatus
  onWorkflow: (id: string | undefined) => void
  onScratch: (on: boolean) => void
  onStatus: (s: string | undefined) => void
}

function Filters({ workflowId, scratch, status, onWorkflow, onScratch, onStatus }: FiltersProps) {
  const workflows = useWorkflows()

  return (
    <div className="mx-auto flex w-full max-w-4xl flex-wrap items-center gap-2 px-4 pt-6 pb-3 md:px-6">
      <Select
        aria-label="Workflow"
        className="w-auto max-w-64"
        value={workflowId ?? ''}
        onChange={(e) => onWorkflow(e.target.value || undefined)}
      >
        <option value="">All workflows</option>
        {workflows.data?.map((w) => (
          <option key={w.id} value={w.id}>
            {w.name}
          </option>
        ))}
      </Select>
      <Select
        aria-label="Status"
        className="w-auto"
        value={status ?? ''}
        onChange={(e) => onStatus(e.target.value || undefined)}
      >
        <option value="">Any status</option>
        {STATUSES.map((s) => (
          <option key={s} value={s}>
            {statusLabel(s)}
          </option>
        ))}
      </Select>
      <Button
        variant="secondary"
        aria-pressed={scratch}
        className="aria-pressed:border-brass/70 aria-pressed:text-brass"
        onClick={() => onScratch(!scratch)}
      >
        Scratch only
      </Button>
    </div>
  )
}

function RunRow({ run, index }: { run: RunSummary; index: number }) {
  const now = useNow(isActive(run.status))
  const took = elapsed(run.startedAt, run.finishedAt, now)

  return (
    <motion.li
      initial={{ opacity: 0, y: 4 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: Math.min(index, 10) * 0.02, duration: 0.25 }}
      className="relative flex items-center gap-3 px-4 py-3 transition-colors hover:bg-raised"
    >
      <StatusIcon status={run.status} />
      <div className="min-w-0 flex-1">
        <Link
          to={`/runs/${run.id}`}
          className="font-medium text-fg after:absolute after:inset-0 focus-visible:outline-none focus-visible:after:outline-2 focus-visible:after:-outline-offset-2 focus-visible:after:outline-brass"
        >
          {run.workflowName}
        </Link>
        {!run.workflowId && (
          <span className="ml-2 rounded-full bg-hover px-1.5 py-0.5 text-[11px] text-fg-subtle">
            scratch
          </span>
        )}
        <p className="truncate text-xs text-fg-subtle">
          {statusLabel(run.status)} ·{' '}
          {/* Above the row link so the id can be selected and copied. */}
          <span className="relative font-mono select-all">{run.id}</span>
          {run.error && ` · ${run.error}`}
        </p>
      </div>
      <span className="hidden font-mono text-xs text-fg-subtle sm:inline">
        {run.stepCount} {run.stepCount === 1 ? 'step' : 'steps'}
      </span>
      <span className="w-16 text-right font-mono text-xs text-fg-muted tabular-nums">
        {took !== null ? formatDuration(took) : '—'}
      </span>
      <span
        className="shrink-0 text-xs whitespace-nowrap text-fg-subtle"
        title={new Date(run.createdAt).toLocaleString()}
      >
        {timeAgo(run.createdAt)}
      </span>
    </motion.li>
  )
}

function Centered({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-1 items-center justify-center px-4">
      <div className="max-w-sm text-center">
        <h2 className="text-base font-medium text-fg">{title}</h2>
        <p className="mt-2 text-fg-muted">{children}</p>
      </div>
    </section>
  )
}
