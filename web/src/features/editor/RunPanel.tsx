import { ArrowUpRight, X } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link } from 'react-router'
import { useShallow } from 'zustand/react/shallow'

import { isActive, useRun, useRunLogs, type Run } from '@/features/runs/api'
import { LogView } from '@/features/runs/LogView'
import { StatusBadge, StatusIcon } from '@/features/runs/StatusIcon'
import { useRunStream } from '@/features/runs/stream'
import { cn } from '@/lib/cn'
import { elapsed, formatDuration } from '@/lib/time'
import { useNow } from '@/lib/useNow'

import { useEditor } from './store'

/**
 * A run started from the editor without leaving it: its status, and the logs
 * of the step selected here or on the canvas.
 */
export function RunPanel({ runId, onClose }: { runId: string; onClose: () => void }) {
  const run = useRun(runId).data
  const active = !!run && isActive(run.status)
  useRunStream(runId, active)
  const logs = useRunLogs(runId).data ?? []
  const now = useNow(active)

  // The step picked in the panel wins until the canvas selection changes.
  const selected = useEditor(useShallow((s) => s.nodes.filter((n) => n.selected).map((n) => n.id)))
  const canvasPick = selected.length === 1 ? selected[0] : undefined
  const [pick, setPick] = useState<{ step: string; canvas: string | undefined }>()
  const picked = pick && pick.canvas === canvasPick ? pick.step : undefined

  const auto = run && defaultStep(run)
  const stepId =
    picked ?? (canvasPick && run?.steps.some((s) => s.stepId === canvasPick) ? canvasPick : auto)
  const step = run?.steps.find((s) => s.stepId === stepId)
  const names = useMemo(
    () => new Map((run?.definition.steps ?? []).map((s) => [s.id, s.name || s.id])),
    [run?.definition.steps],
  )

  if (!run) return null
  const took = elapsed(run.startedAt, run.finishedAt, now)

  return (
    <section
      aria-label="Test run"
      className="@container flex h-64 shrink-0 animate-fade-in flex-col border-t border-line bg-surface"
    >
      <div className="flex h-10 shrink-0 items-center gap-2.5 border-b border-line px-3">
        <span className="text-[11px] font-medium tracking-wide text-fg-subtle uppercase">
          Test run
        </span>
        <StatusBadge status={run.status} />
        {took !== null && (
          <span className="font-mono text-xs text-fg-subtle tabular-nums">
            {formatDuration(took)}
          </span>
        )}
        {run.error && <span className="min-w-0 truncate text-xs text-coral">{run.error}</span>}
        <Link
          to={`/runs/${run.id}`}
          className="ml-auto flex items-center gap-1 rounded-md px-2 py-1 text-xs text-fg-muted hover:bg-hover hover:text-fg"
        >
          Open run <ArrowUpRight className="size-3" />
        </Link>
        <button
          aria-label="Close test run"
          onClick={onClose}
          className="flex size-7 items-center justify-center rounded-md text-fg-subtle hover:bg-hover hover:text-fg"
        >
          <X className="size-3.5" />
        </button>
      </div>
      <div className="flex min-h-0 flex-1">
        <ol className="hidden w-48 shrink-0 overflow-y-auto border-r border-line p-1.5 @lg:block">
          {run.steps.map((s) => (
            <li key={s.stepId}>
              <button
                aria-current={s.stepId === stepId ? 'true' : undefined}
                onClick={() => setPick({ step: s.stepId, canvas: canvasPick })}
                className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs transition-colors hover:bg-hover/60 aria-[current=true]:bg-hover"
              >
                <StatusIcon status={s.status} className="size-3.5" />
                <span className="truncate text-fg">{names.get(s.stepId)}</span>
              </button>
            </li>
          ))}
        </ol>
        <div className="flex min-w-0 flex-1 flex-col">
          {step?.error && (
            <p
              className={cn(
                'shrink-0 border-b border-line px-4 py-1.5 text-xs',
                step.status === 'failed' ? 'text-coral' : 'text-fg-muted',
              )}
            >
              {step.error}
            </p>
          )}
          <div className="min-h-0 flex-1 overflow-hidden [&_[role=log]]:h-full [&_[role=log]]:max-h-full">
            <LogView
              lines={logs.filter((l) => l.stepId === stepId)}
              follow={step?.status === 'running'}
              empty={
                step?.status === 'queued'
                  ? 'Waiting for its dependencies.'
                  : step?.status === 'skipped' || step?.status === 'cancelled'
                    ? 'This step did not run.'
                    : 'No output.'
              }
            />
          </div>
        </div>
      </div>
    </section>
  )
}

/** What to show first: a failure, then what's running or waiting, then the last step. */
function defaultStep(run: Run) {
  return (
    run.steps.find((s) => s.status === 'failed')?.stepId ??
    run.steps.find((s) => s.status === 'running' || s.status === 'waiting')?.stepId ??
    run.steps.findLast((s) => s.status !== 'queued')?.stepId ??
    run.steps[0]?.stepId
  )
}
