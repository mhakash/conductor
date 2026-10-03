import { ChevronLeft, Play, Square, Workflow as WorkflowIcon } from 'lucide-react'
import { useMemo, type ReactNode } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router'
import { toast } from 'sonner'

import { Button } from '@/components/ui/Button'
import { ApiError } from '@/lib/api'
import { cn } from '@/lib/cn'
import { elapsed, formatDuration, timeAgo } from '@/lib/time'
import { useNow } from '@/lib/useNow'

import {
  isActive,
  useCancelRun,
  useRun,
  useRunLogs,
  useStartRun,
  type LogLine,
  type Run,
  type StepRun,
} from './api'
import { LogView } from './LogView'
import { executionOrder } from './order'
import { StatusBadge, StatusIcon } from './StatusIcon'

export function RunPage() {
  const { id = '' } = useParams()
  const run = useRun(id)

  if (run.isError) {
    const notFound = run.error instanceof ApiError && run.error.status === 404
    return (
      <div className="flex flex-1 items-center justify-center px-4 text-center">
        <div>
          <h2 className="text-base font-medium text-fg">
            {notFound ? 'This run doesn’t exist' : 'Couldn’t load the run'}
          </h2>
          <p className="mt-2 text-fg-muted">
            {!notFound && `${run.error.message}. `}
            <Link to="/runs" className="text-brass hover:underline">
              Back to runs
            </Link>
          </p>
        </div>
      </div>
    )
  }
  if (!run.data) return null
  return <RunView run={run.data} />
}

function RunView({ run }: { run: Run }) {
  const active = isActive(run.status)
  const logs = useRunLogs(run.id, active)
  const now = useNow(active)
  const [params, setParams] = useSearchParams()

  const order = useMemo(() => executionOrder(run.definition), [run.definition])
  const stepRuns = useMemo(() => new Map(run.steps.map((s) => [s.stepId, s])), [run.steps])
  const defs = useMemo(
    () => new Map(run.definition.steps.map((s) => [s.id, s])),
    [run.definition.steps],
  )

  // Explicit choice first; otherwise follow what's most interesting right now.
  const selectedId =
    params.get('step') ??
    run.steps.find((s) => s.status === 'failed')?.stepId ??
    run.steps.find((s) => s.status === 'running')?.stepId ??
    order[0]?.id
  const selected = selectedId ? stepRuns.get(selectedId) : undefined

  const took = elapsed(run.startedAt, run.finishedAt, now)

  return (
    <div className="flex h-full flex-col">
      <RunHeader run={run} took={took} />

      {run.error && (
        <p className="border-b border-line bg-coral/5 px-4 py-2.5 text-coral md:px-6">
          {run.error}
        </p>
      )}

      <div className="flex min-h-0 flex-1 flex-col md:flex-row">
        <nav
          aria-label="Steps"
          className="max-h-60 shrink-0 overflow-y-auto border-b border-line bg-surface p-2 md:max-h-none md:w-72 md:border-r md:border-b-0"
        >
          <ol className="flex flex-col gap-0.5">
            {order.map(({ id, after }) => {
              const sr = stepRuns.get(id)
              if (!sr) return null
              const d = elapsed(sr.startedAt, sr.finishedAt, now)
              return (
                <li key={id}>
                  <button
                    aria-current={id === selectedId ? 'true' : undefined}
                    onClick={() => setParams({ step: id }, { replace: true })}
                    className="flex w-full items-start gap-2.5 rounded-lg px-2.5 py-2 text-left transition-colors hover:bg-hover/60 aria-[current=true]:bg-hover"
                  >
                    <StatusIcon status={sr.status} className="mt-0.5" />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-fg">{defs.get(id)?.name || id}</span>
                      <span className="block truncate font-mono text-[11px] text-fg-subtle">
                        {id}
                        {after.length > 0 && ` · after ${after.join(', ')}`}
                      </span>
                    </span>
                    <span className="mt-0.5 font-mono text-[11px] text-fg-subtle tabular-nums">
                      {d !== null && formatDuration(d)}
                    </span>
                  </button>
                </li>
              )
            })}
          </ol>
        </nav>

        <section className="min-w-0 flex-1 overflow-y-auto">
          {selected && (
            <StepDetail
              key={selected.stepId}
              step={selected}
              name={defs.get(selected.stepId)?.name || selected.stepId}
              block={defs.get(selected.stepId)?.block ?? ''}
              lines={(logs.data ?? []).filter((l) => l.stepId === selected.stepId)}
              now={now}
            />
          )}
        </section>
      </div>
    </div>
  )
}

function RunHeader({ run, took }: { run: Run; took: number | null }) {
  const cancel = useCancelRun()
  const start = useStartRun()
  const navigate = useNavigate()
  const active = isActive(run.status)

  const runAgain = () =>
    start.mutate(run.workflowId, {
      onSuccess: (next) => navigate(`/runs/${next.id}`),
      onError: (err) => toast.error(`Couldn't start the run: ${err.message}`),
    })

  return (
    <header className="flex min-h-14 shrink-0 flex-wrap items-center gap-x-3 gap-y-2 border-b border-line px-3 py-2 md:px-4">
      <Link
        to="/runs"
        aria-label="All runs"
        className="flex size-8 items-center justify-center rounded-md text-fg-muted hover:bg-hover hover:text-fg"
      >
        <ChevronLeft className="size-4" />
      </Link>
      <h1 className="text-sm font-medium text-fg">{run.workflowName}</h1>
      <StatusBadge status={run.status} />
      <span className="text-xs text-fg-subtle">
        Started {timeAgo(run.createdAt)}
        {took !== null && (
          <>
            {' · '}
            <span className="font-mono tabular-nums">{formatDuration(took)}</span>
          </>
        )}
      </span>

      <div className="ml-auto flex items-center gap-1.5">
        <Link
          to={`/workflows/${run.workflowId}`}
          className="inline-flex h-8 items-center gap-1.5 rounded-md px-3 text-[13px] font-medium text-fg-muted hover:bg-hover hover:text-fg"
        >
          <WorkflowIcon className="size-3.5" />
          <span className="hidden sm:inline">Open workflow</span>
        </Link>
        {active ? (
          <Button
            variant="secondary"
            disabled={cancel.isPending}
            onClick={() =>
              cancel.mutate(run.id, {
                onError: (err) => toast.error(`Couldn't cancel: ${err.message}`),
              })
            }
          >
            <Square className="size-3" fill="currentColor" />
            Cancel run
          </Button>
        ) : (
          <Button onClick={runAgain} disabled={start.isPending}>
            <Play className="size-3" fill="currentColor" />
            Run again
          </Button>
        )}
      </div>
    </header>
  )
}

type StepDetailProps = {
  step: StepRun
  name: string
  block: string
  lines: LogLine[]
  now: number
}

function StepDetail({ step, name, block, lines, now }: StepDetailProps) {
  const d = elapsed(step.startedAt, step.finishedAt, now)
  const outputs = Object.entries(step.outputs ?? {})
  const empty =
    step.status === 'queued'
      ? 'Waiting for its dependencies.'
      : step.status === 'skipped' || step.status === 'cancelled'
        ? 'This step did not run.'
        : 'No output.'

  return (
    <div className="flex flex-col gap-5 p-4 md:p-6">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <h2 className="text-base font-medium text-fg">{name}</h2>
        <StatusBadge status={step.status} />
        <span className="font-mono text-xs text-fg-subtle">
          {block} · {step.stepId}
          {d !== null && ` · ${formatDuration(d)}`}
        </span>
      </div>

      {step.error && (
        <p
          className={cn(
            'rounded-lg border px-3 py-2',
            step.status === 'failed'
              ? 'border-coral/30 bg-coral/5 text-coral'
              : 'border-line bg-raised text-fg-muted',
          )}
        >
          {step.error}
        </p>
      )}

      <Panel title="Logs">
        <LogView lines={lines} follow={step.status === 'running'} empty={empty} />
      </Panel>

      {outputs.length > 0 && (
        <Panel title="Outputs">
          <dl className="divide-y divide-line">
            {outputs.map(([key, value]) => (
              <div key={key} className="flex flex-col gap-1 px-4 py-2.5 sm:flex-row sm:gap-4">
                <dt className="w-28 shrink-0 font-mono text-xs text-fg-muted">{key}</dt>
                <dd className="min-w-0 flex-1">
                  <OutputValue value={value} />
                </dd>
              </div>
            ))}
          </dl>
        </Panel>
      )}
    </div>
  )
}

function OutputValue({ value }: { value: unknown }) {
  if (value === '' || value === null) return <span className="text-xs text-fg-subtle">empty</span>
  const text = typeof value === 'string' ? value : JSON.stringify(value, null, 2)
  return (
    <pre className="max-h-64 overflow-auto font-mono text-xs break-words whitespace-pre-wrap text-fg">
      {text}
    </pre>
  )
}

function Panel({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="overflow-hidden rounded-xl border border-line bg-surface">
      <h3 className="border-b border-line px-4 py-2 text-[11px] font-medium tracking-wide text-fg-subtle uppercase">
        {title}
      </h3>
      {children}
    </section>
  )
}
