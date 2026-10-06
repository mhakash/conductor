import {
  ArrowUpRight,
  ChevronLeft,
  CornerLeftUp,
  Hand,
  Play,
  Square,
  Workflow as WorkflowIcon,
} from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
import { Link, useParams, useSearchParams } from 'react-router'
import { toast } from 'sonner'

import { Button } from '@/components/ui/Button'
import { Textarea } from '@/components/ui/Input'
import { CALL_BLOCK } from '@/features/blocks/api'
import type { Step } from '@/features/workflows/api'
import { ApiError } from '@/lib/api'
import { cn } from '@/lib/cn'
import { elapsed, formatDuration, timeAgo } from '@/lib/time'
import { useNow } from '@/lib/useNow'

import {
  isActive,
  statusLabel,
  useCancelRun,
  useDecide,
  useRun,
  useRunLogs,
  type LogLine,
  type Run,
  type RunSummary,
  type StepRun,
} from './api'
import { LogView } from './LogView'
import { RunGraph } from './RunGraph'
import { useRunStream } from './stream'
import { executionOrder } from './order'
import { StatusBadge, StatusIcon } from './StatusIcon'
import { stepSummary, withChildWaits } from './summary'
import { useRunWorkflow } from './useRunWorkflow'

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
  const { live } = useRunStream(run.id, active)
  const logs = useRunLogs(run.id)
  const now = useNow(active)
  const [params, setParams] = useSearchParams()
  const select = (id: string) => setParams({ step: id }, { replace: true })

  const order = useMemo(() => executionOrder(run.definition), [run.definition])
  // Steps that run a workflow show as waiting while that run waits for approval.
  const steps = useMemo(() => withChildWaits(run.steps, run.children), [run.steps, run.children])
  const shown = useMemo(() => (steps === run.steps ? run : { ...run, steps }), [run, steps])
  const stepRuns = useMemo(() => new Map(steps.map((s) => [s.stepId, s])), [steps])
  const defs = useMemo(
    () => new Map(run.definition.steps.map((s) => [s.id, s])),
    [run.definition.steps],
  )

  // Explicit choice first; otherwise follow what's most interesting right now:
  // a failure, a step waiting for you, the step in progress, or — once done —
  // where the run ended.
  const waiting = steps.find((s) => s.status === 'waiting')
  const selectedId =
    params.get('step') ??
    steps.find((s) => s.status === 'failed' && !defs.get(s.stepId)?.continueOnError)?.stepId ??
    waiting?.stepId ??
    steps.findLast((s) => s.status === 'running')?.stepId ??
    (active ? order[0]?.id : order.at(-1)?.id)
  const selected = selectedId ? stepRuns.get(selectedId) : undefined

  const took = elapsed(run.startedAt, run.finishedAt, now)

  return (
    <div className="flex h-full flex-col">
      <RunHeader run={run} took={took} live={live} />

      <Values title="Inputs" values={run.inputs} />
      <Values title="Outputs" values={run.outputs} />

      {run.error && (
        <p className="border-b border-line bg-coral/5 px-4 py-2.5 text-coral md:px-6">
          {run.error}
        </p>
      )}

      {waiting && waiting.stepId !== selectedId && (
        <div className="flex animate-fade-in items-center gap-3 border-b border-line bg-brass/5 px-4 py-2 md:px-6">
          <Hand className="size-4 shrink-0 text-brass" strokeWidth={1.75} />
          <p className="min-w-0 flex-1 truncate text-fg">
            <span className="font-medium">{defs.get(waiting.stepId)?.name || waiting.stepId}</span>{' '}
            <span className="text-fg-muted">is waiting for your approval.</span>
          </p>
          <Button variant="secondary" onClick={() => select(waiting.stepId)}>
            Review
          </Button>
        </div>
      )}

      <section
        aria-label="Run graph"
        className="relative h-[40%] min-h-56 shrink-0 border-b border-line"
      >
        <RunGraph run={shown} selectedId={selectedId} onSelect={select} />
      </section>

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
                    onClick={() => select(id)}
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
              def={defs.get(selected.stepId)}
              runId={run.id}
              lines={(logs.data ?? []).filter((l) => l.stepId === selected.stepId)}
              subRuns={run.children
                .filter((c) => c.parent?.stepId === selected.stepId)
                .sort((a, b) => (a.parent?.item ?? 0) - (b.parent?.item ?? 0))}
              now={now}
            />
          )}
        </section>
      </div>
    </div>
  )
}

/** A run's inputs or outputs, on one line under the header. */
function Values({ title, values }: { title: string; values: Record<string, unknown> | undefined }) {
  if (!values || Object.keys(values).length === 0) return null
  return (
    <dl className="flex flex-wrap items-center gap-x-4 gap-y-1 border-b border-line px-4 py-2 md:px-6">
      <dt className="text-[11px] font-medium tracking-wide text-fg-subtle uppercase">{title}</dt>
      {Object.entries(values).map(([k, v]) => (
        <dd key={k} className="max-w-full truncate font-mono text-xs">
          <span className="text-fg-subtle">{k}=</span>
          <span className="text-fg">{typeof v === 'string' ? v : JSON.stringify(v)}</span>
        </dd>
      ))}
    </dl>
  )
}

function RunHeader({ run, took, live }: { run: Run; took: number | null; live: boolean }) {
  const cancel = useCancelRun()
  const runner = useRunWorkflow()
  const active = isActive(run.status)

  // Runs again with the same inputs, shown for review when there are any.
  const runAgain = () =>
    runner.run({
      workflowId: run.workflowId,
      workflowName: run.workflowName,
      inputs: run.definition.inputs ?? [],
      initial: run.inputs,
    })

  return (
    <header className="flex min-h-14 shrink-0 flex-wrap items-center gap-x-3 gap-y-2 border-b border-line px-3 py-2 md:px-4">
      <Link
        to={run.parent ? parentLink(run.parent) : '/runs'}
        aria-label={run.parent ? `Back to ${run.parent.workflowName}` : 'All runs'}
        className="flex size-8 items-center justify-center rounded-md text-fg-muted hover:bg-hover hover:text-fg"
      >
        <ChevronLeft className="size-4" />
      </Link>
      <h1 className="text-sm font-medium text-fg">{run.workflowName}</h1>
      {run.parent && (
        <Link
          to={parentLink(run.parent)}
          className="flex items-center gap-1 text-xs text-fg-subtle hover:text-fg"
        >
          <CornerLeftUp className="size-3" />
          Run by <span className="text-fg-muted">{run.parent.workflowName}</span>
          <span className="font-mono">
            · {run.parent.stepId}
            {run.parent.item !== undefined && ` #${run.parent.item + 1}`}
          </span>
        </Link>
      )}
      <StatusBadge status={run.status} />
      {live && (
        <span className="flex items-center gap-1.5 text-xs text-brass" title="Updating live">
          <span className="size-1.5 animate-pulse rounded-full bg-brass motion-reduce:animate-none" />
          Live
        </span>
      )}
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
        ) : run.parent ? null : (
          <Button onClick={runAgain} disabled={runner.pending}>
            <Play className="size-3" fill="currentColor" />
            Run again
          </Button>
        )}
      </div>
      {runner.dialog}
    </header>
  )
}

function parentLink(p: NonNullable<Run['parent']>) {
  return `/runs/${p.runId}?step=${p.stepId}`
}

type StepDetailProps = {
  step: StepRun
  name: string
  block: string
  def: Step | undefined
  runId: string
  lines: LogLine[]
  /** Runs the step started, for a step that runs a workflow. */
  subRuns: RunSummary[]
  now: number
}

function StepDetail({ step, name, block, def, runId, lines, subRuns, now }: StepDetailProps) {
  const d = elapsed(step.startedAt, step.finishedAt, now)
  const outputs = Object.entries(step.outputs ?? {})
  // The config after expressions were evaluated (secrets masked); one per
  // item for a step that runs once per item.
  const configs = Array.isArray(step.config) ? step.config : [step.config ?? {}]
  const progress = step.status === 'running' ? stepSummary(step, def) : null
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
          {progress && progress !== 'Running' && ` · ${progress}`}
        </span>
      </div>

      {step.status === 'waiting' && block !== CALL_BLOCK && (
        <ApprovalCard
          runId={runId}
          stepId={step.stepId}
          message={(configs[0]?.message as string | undefined) ?? ''}
          since={step.startedAt}
        />
      )}

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
          {step.status === 'failed' && def?.continueOnError && (
            <span className="mt-0.5 block text-xs text-fg-muted">
              This step is allowed to fail, so the steps after it ran anyway.
            </span>
          )}
        </p>
      )}

      {block === CALL_BLOCK && subRuns.length > 0 && (
        <Panel title={subRuns.length === 1 ? 'Run' : `Runs · ${subRuns.length}`}>
          <ChildRuns runs={subRuns} now={now} />
        </Panel>
      )}

      <Panel title="Logs">
        <LogView lines={lines} follow={step.status === 'running'} empty={empty} />
      </Panel>

      {configs.length === 1 && Object.keys(configs[0]).length > 0 && (
        <Panel title={step.status === 'waiting' ? 'Waiting with' : 'Ran with'}>
          <KeyValues entries={Object.entries(configs[0])} />
        </Panel>
      )}
      {configs.length > 1 && (
        <Panel title={`Ran with · ${configs.length} items`}>
          <div className="divide-y divide-line">
            {configs.map((c, i) => (
              <div key={i}>
                <p className="px-4 pt-2.5 font-mono text-[11px] text-fg-subtle">#{i + 1}</p>
                <KeyValues entries={Object.entries(c)} />
              </div>
            ))}
          </div>
        </Panel>
      )}

      {outputs.length > 0 && (
        <Panel title="Outputs">
          <KeyValues entries={outputs} />
        </Panel>
      )}
    </div>
  )
}

function ChildRuns({ runs, now }: { runs: RunSummary[]; now: number }) {
  return (
    <ul className="divide-y divide-line">
      {runs.map((r) => {
        const d = elapsed(r.startedAt, r.finishedAt, now)
        return (
          <li key={r.id} className="relative flex items-center gap-2.5 px-4 py-2.5 hover:bg-raised">
            <StatusIcon status={r.status} />
            <span className="min-w-0 flex-1">
              <Link
                to={`/runs/${r.id}`}
                className="text-fg after:absolute after:inset-0 focus-visible:outline-none focus-visible:after:outline-2 focus-visible:after:-outline-offset-2 focus-visible:after:outline-brass"
              >
                {r.workflowName}
                {r.parent?.item !== undefined && (
                  <span className="ml-1.5 font-mono text-xs text-fg-subtle">
                    #{r.parent.item + 1}
                  </span>
                )}
              </Link>
              <span
                className={cn(
                  'block truncate text-xs',
                  r.status === 'waiting' ? 'text-brass' : 'text-fg-subtle',
                )}
              >
                {r.error || statusLabel(r.status)}
              </span>
            </span>
            <span className="font-mono text-[11px] text-fg-subtle tabular-nums">
              {d !== null && formatDuration(d)}
            </span>
            <ArrowUpRight className="size-3.5 text-fg-subtle" />
          </li>
        )
      })}
    </ul>
  )
}

function ApprovalCard({
  runId,
  stepId,
  message,
  since,
}: {
  runId: string
  stepId: string
  message: string
  since: string | null
}) {
  const [comment, setComment] = useState('')
  const decide = useDecide(runId)

  const submit = (approve: boolean) =>
    decide.mutate(
      { stepId, approve, comment: comment.trim() },
      {
        onSuccess: () => toast(approve ? 'Approved' : 'Rejected'),
        onError: (err) => toast.error(`Couldn't send your decision: ${err.message}`),
      },
    )

  return (
    <section
      aria-label="Approval"
      className="flex animate-pop-in flex-col gap-3 rounded-xl border border-brass/40 bg-brass/5 p-4"
    >
      <p className="flex items-center gap-2 text-[11px] font-medium tracking-wide text-brass uppercase">
        <Hand className="size-3.5" strokeWidth={2} />
        Waiting for your approval
        {since && (
          <span className="font-normal tracking-normal text-fg-subtle normal-case">
            · since {timeAgo(since)}
          </span>
        )}
      </p>
      <p className="text-[15px] whitespace-pre-wrap text-fg">
        {message || 'Approve to let the run continue.'}
      </p>
      <Textarea
        aria-label="Comment"
        placeholder="Comment (optional) — later steps can read it"
        value={comment}
        rows={2}
        onChange={(e) => setComment(e.target.value)}
      />
      <div className="flex flex-wrap items-center gap-2">
        <Button onClick={() => submit(true)} disabled={decide.isPending}>
          Approve
        </Button>
        <Button variant="danger" onClick={() => submit(false)} disabled={decide.isPending}>
          Reject
        </Button>
        <span className="text-xs text-fg-subtle">Rejecting fails this step.</span>
      </div>
    </section>
  )
}

function KeyValues({ entries }: { entries: [string, unknown][] }) {
  return (
    <dl className="divide-y divide-line">
      {entries.map(([key, value]) => (
        <div key={key} className="flex flex-col gap-1 px-4 py-2.5 sm:flex-row sm:gap-4">
          <dt className="w-28 shrink-0 font-mono text-xs text-fg-muted">{key}</dt>
          <dd className="min-w-0 flex-1">
            <OutputValue value={value} />
          </dd>
        </div>
      ))}
    </dl>
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
