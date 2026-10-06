import { motion } from 'motion/react'
import type { ReactNode } from 'react'
import { Link } from 'react-router'

import { PageHeader } from '@/components/layout/PageHeader'
import { elapsed, formatDuration, timeAgo } from '@/lib/time'
import { useNow } from '@/lib/useNow'

import { isActive, statusLabel, useRuns, type RunSummary } from './api'
import { StatusIcon } from './StatusIcon'

export function RunsPage() {
  const runs = useRuns()

  return (
    <>
      <PageHeader title="Runs" />
      <div className="flex flex-1 flex-col overflow-y-auto">
        {runs.isPending ? null : runs.isError ? (
          <Centered title="Couldn't load runs">{runs.error.message}</Centered>
        ) : runs.data.length === 0 ? (
          <Centered title="No runs yet">
            Run a workflow from its editor. Each run shows up here with its logs, outputs and step
            timings.
          </Centered>
        ) : (
          <div className="mx-auto w-full max-w-4xl px-4 py-6 md:px-6">
            <ul className="divide-y divide-line overflow-hidden rounded-xl border border-line bg-surface">
              {runs.data.map((r, i) => (
                <RunRow key={r.id} run={r} index={i} />
              ))}
            </ul>
          </div>
        )}
      </div>
    </>
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
          {statusLabel(run.status)}
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
